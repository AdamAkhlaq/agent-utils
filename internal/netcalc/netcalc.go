// Package netcalc does exact CIDR/subnet arithmetic on IPv4 and IPv6
// prefixes: describing a network, membership and overlap tests, and splitting
// a prefix into smaller subnets. Address counts use math/big because IPv6
// totals overflow every fixed-width integer type.
//
// Family behavior follows net/netip exactly: a prefix or address in one
// family never contains or overlaps one in the other, and that includes
// IPv4-mapped IPv6 forms ("::ffff:10.0.0.1" is IPv6, not IPv4).
package netcalc

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/netip"
	"strings"
)

// Details describes one prefix. TotalAddresses is a decimal string because
// IPv6 counts (up to 2^128) overflow both int64 and float64, and a JSON
// number would be silently corrupted by most consumers. Netmask and
// UsableHosts are IPv4-only concepts and are omitted for IPv6: IPv6 has no
// dotted netmask notation and no broadcast address, so every address counts
// as usable there.
type Details struct {
	Input          string `json:"input"`
	Network        string `json:"network"`
	HostBitsSet    bool   `json:"host_bits_set"`
	PrefixLength   int    `json:"prefix_length"`
	Netmask        string `json:"netmask,omitempty"`
	FirstAddress   string `json:"first_address"`
	LastAddress    string `json:"last_address"`
	UsableHosts    uint64 `json:"usable_hosts,omitempty"`
	TotalAddresses string `json:"total_addresses"`
}

// Info parses prefix and describes its network. The input may have host bits
// set (10.0.5.1/24); the reported network is the masked form and HostBitsSet
// records that they were nonzero. UsableHosts follows convention: for IPv4
// prefixes up to /30 the network and broadcast addresses are excluded (total
// minus 2), a /31 has 2 usable hosts (RFC 3021 point-to-point), and a /32 is
// a single host route with 1.
func Info(prefix string) (Details, error) {
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return Details{}, err
	}
	masked := p.Masked()
	d := Details{
		Input:          prefix,
		Network:        masked.String(),
		HostBitsSet:    p.Addr() != masked.Addr(),
		PrefixLength:   p.Bits(),
		FirstAddress:   masked.Addr().String(),
		LastAddress:    lastAddr(masked).String(),
		TotalAddresses: totalAddresses(p).String(),
	}
	if p.Addr().Is4() {
		d.Netmask = netmask4(p.Bits())
		d.UsableHosts = usable4(p.Bits())
	}
	return d, nil
}

// JSON renders d as pretty-printed JSON.
func JSON(d Details) (string, error) {
	out, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding JSON: %w", err)
	}
	return string(out), nil
}

// Contains reports whether ip lies inside prefix. Host bits set on the
// prefix are masked off first. A mismatched address family (an IPv6 address
// against an IPv4 prefix, or vice versa) is false, not an error, and an IPv6
// address with a zone is never contained; both follow net/netip.
func Contains(prefix, ip string) (bool, error) {
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return false, err
	}
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return false, err
	}
	return p.Masked().Contains(a), nil
}

// Overlaps reports whether the two prefixes share any address. Host bits set
// on either prefix are masked off first; prefixes of different address
// families never overlap.
func Overlaps(a, b string) (bool, error) {
	pa, err := netip.ParsePrefix(a)
	if err != nil {
		return false, fmt.Errorf("first prefix: %w", err)
	}
	pb, err := netip.ParsePrefix(b)
	if err != nil {
		return false, fmt.Errorf("second prefix: %w", err)
	}
	return pa.Masked().Overlaps(pb.Masked()), nil
}

// splitLimit caps how many subnets Split will produce. 65536 lines is
// already generous for terminal or pipe output; anything larger is almost
// certainly a mistyped prefix length, and printing it would bury the caller.
const splitLimit = 65536

// Split divides prefix into all its subnets of newLen bits, in address
// order. newLen must be at least the prefix's own length and at most the
// family's address width; host bits set on the input are masked off first.
// More than 65536 resulting subnets is refused with the exact count.
func Split(prefix string, newLen int) ([]string, error) {
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, err
	}
	masked := p.Masked()
	width := p.Addr().BitLen()
	if newLen < p.Bits() || newLen > width {
		return nil, fmt.Errorf("new prefix length %d is out of range: must be between %d (the prefix's own length) and %d", newLen, p.Bits(), width)
	}
	extraBits := newLen - p.Bits()
	if count := new(big.Int).Lsh(big.NewInt(1), uint(extraBits)); count.Cmp(big.NewInt(splitLimit)) > 0 {
		return nil, fmt.Errorf("splitting %s into /%d would produce %s subnets; refusing to print more than %d", masked, newLen, count, splitLimit)
	}
	count := 1 << extraBits
	subnets := make([]string, 0, count)
	addr := masked.Addr()
	for i := 0; i < count; i++ {
		sub := netip.PrefixFrom(addr, newLen)
		subnets = append(subnets, sub.String())
		if i < count-1 {
			addr = lastAddr(sub).Next()
		}
	}
	return subnets, nil
}

// lastAddr returns the highest address in p (the broadcast address for IPv4
// prefixes up to /30).
func lastAddr(p netip.Prefix) netip.Addr {
	if p.Addr().Is4() {
		b := p.Masked().Addr().As4()
		setHostBits(b[:], p.Bits())
		return netip.AddrFrom4(b)
	}
	b := p.Masked().Addr().As16()
	setHostBits(b[:], p.Bits())
	return netip.AddrFrom16(b)
}

func setHostBits(b []byte, prefixLen int) {
	for i := prefixLen; i < len(b)*8; i++ {
		b[i/8] |= 1 << (7 - i%8)
	}
}

func totalAddresses(p netip.Prefix) *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), uint(p.Addr().BitLen()-p.Bits()))
}

func usable4(prefixLen int) uint64 {
	switch prefixLen {
	case 32:
		return 1
	case 31:
		return 2
	default:
		return 1<<(32-prefixLen) - 2
	}
}

func netmask4(prefixLen int) string {
	// A shift count of 32 (prefixLen 0) is defined in Go and yields 0.
	mask := ^uint32(0) << (32 - prefixLen)
	parts := make([]string, 4)
	for i := range parts {
		parts[i] = fmt.Sprintf("%d", byte(mask>>(24-8*i)))
	}
	return strings.Join(parts, ".")
}
