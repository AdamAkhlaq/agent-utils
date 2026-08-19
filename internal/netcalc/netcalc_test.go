package netcalc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		want    Details
		wantErr string
	}{
		{
			name:   "IPv4 /24",
			prefix: "10.0.0.0/24",
			want: Details{
				Input: "10.0.0.0/24", Network: "10.0.0.0/24", HostBitsSet: false,
				PrefixLength: 24, Netmask: "255.255.255.0",
				FirstAddress: "10.0.0.0", LastAddress: "10.0.0.255",
				UsableHosts: 254, TotalAddresses: "256",
			},
		},
		{
			name:   "IPv4 host bits set",
			prefix: "10.0.5.1/24",
			want: Details{
				Input: "10.0.5.1/24", Network: "10.0.5.0/24", HostBitsSet: true,
				PrefixLength: 24, Netmask: "255.255.255.0",
				FirstAddress: "10.0.5.0", LastAddress: "10.0.5.255",
				UsableHosts: 254, TotalAddresses: "256",
			},
		},
		{
			name:   "IPv4 /31 point-to-point",
			prefix: "192.168.1.0/31",
			want: Details{
				Input: "192.168.1.0/31", Network: "192.168.1.0/31", HostBitsSet: false,
				PrefixLength: 31, Netmask: "255.255.255.254",
				FirstAddress: "192.168.1.0", LastAddress: "192.168.1.1",
				UsableHosts: 2, TotalAddresses: "2",
			},
		},
		{
			name:   "IPv4 /32 host route",
			prefix: "192.168.1.5/32",
			want: Details{
				Input: "192.168.1.5/32", Network: "192.168.1.5/32", HostBitsSet: false,
				PrefixLength: 32, Netmask: "255.255.255.255",
				FirstAddress: "192.168.1.5", LastAddress: "192.168.1.5",
				UsableHosts: 1, TotalAddresses: "1",
			},
		},
		{
			name:   "IPv4 /0",
			prefix: "0.0.0.0/0",
			want: Details{
				Input: "0.0.0.0/0", Network: "0.0.0.0/0", HostBitsSet: false,
				PrefixLength: 0, Netmask: "0.0.0.0",
				FirstAddress: "0.0.0.0", LastAddress: "255.255.255.255",
				UsableHosts: 4294967294, TotalAddresses: "4294967296",
			},
		},
		{
			name:   "IPv6 /64 with exact big count",
			prefix: "2001:db8::/64",
			want: Details{
				Input: "2001:db8::/64", Network: "2001:db8::/64", HostBitsSet: false,
				PrefixLength: 64,
				FirstAddress: "2001:db8::", LastAddress: "2001:db8::ffff:ffff:ffff:ffff",
				TotalAddresses: "18446744073709551616",
			},
		},
		{
			name:   "IPv6 host bits set",
			prefix: "2001:db8::1/64",
			want: Details{
				Input: "2001:db8::1/64", Network: "2001:db8::/64", HostBitsSet: true,
				PrefixLength: 64,
				FirstAddress: "2001:db8::", LastAddress: "2001:db8::ffff:ffff:ffff:ffff",
				TotalAddresses: "18446744073709551616",
			},
		},
		{
			name:   "IPv6 /128",
			prefix: "::1/128",
			want: Details{
				Input: "::1/128", Network: "::1/128", HostBitsSet: false,
				PrefixLength: 128,
				FirstAddress: "::1", LastAddress: "::1",
				TotalAddresses: "1",
			},
		},
		{
			name:   "IPv6 /0",
			prefix: "::/0",
			want: Details{
				Input: "::/0", Network: "::/0", HostBitsSet: false,
				PrefixLength: 0,
				FirstAddress: "::", LastAddress: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
				TotalAddresses: "340282366920938463463374607431768211456",
			},
		},
		{name: "prefix length out of range", prefix: "10.0.0.0/33", wantErr: "prefix length out of range"},
		{name: "not an address", prefix: "banana/24", wantErr: "unable to parse IP"},
		{name: "missing slash", prefix: "10.0.0.0", wantErr: "no '/'"},
		{name: "empty", prefix: "", wantErr: "no '/'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Info(tt.prefix)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Info(%q) = %+v, want error containing %q", tt.prefix, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Info(%q) error = %q, want it to contain %q", tt.prefix, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Info(%q) error = %v", tt.prefix, err)
			}
			if got != tt.want {
				t.Errorf("Info(%q) = %+v, want %+v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	t.Run("IPv4 includes netmask and usable hosts", func(t *testing.T) {
		d, err := Info("10.0.5.1/24")
		if err != nil {
			t.Fatal(err)
		}
		out, err := JSON(d)
		if err != nil {
			t.Fatalf("JSON() error = %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(out), &decoded); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, out)
		}
		want := map[string]any{
			"input": "10.0.5.1/24", "network": "10.0.5.0/24", "host_bits_set": true,
			"prefix_length": float64(24), "netmask": "255.255.255.0",
			"first_address": "10.0.5.0", "last_address": "10.0.5.255",
			"usable_hosts": float64(254), "total_addresses": "256",
		}
		for key, w := range want {
			if decoded[key] != w {
				t.Errorf("JSON field %q = %v, want %v", key, decoded[key], w)
			}
		}
	})
	t.Run("IPv6 omits netmask and usable hosts and keeps count a string", func(t *testing.T) {
		d, err := Info("2001:db8::/64")
		if err != nil {
			t.Fatal(err)
		}
		out, err := JSON(d)
		if err != nil {
			t.Fatalf("JSON() error = %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(out), &decoded); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, out)
		}
		for _, key := range []string{"netmask", "usable_hosts"} {
			if _, ok := decoded[key]; ok {
				t.Errorf("JSON field %q present for IPv6, want omitted", key)
			}
		}
		if decoded["total_addresses"] != "18446744073709551616" {
			t.Errorf("total_addresses = %v, want the exact decimal string", decoded["total_addresses"])
		}
	})
}

func TestContains(t *testing.T) {
	tests := []struct {
		name       string
		prefix, ip string
		want       bool
		wantErr    string
	}{
		{name: "inside", prefix: "10.0.0.0/24", ip: "10.0.0.200", want: true},
		{name: "outside", prefix: "10.0.0.0/24", ip: "10.0.1.1", want: false},
		{name: "network address counts", prefix: "10.0.0.0/24", ip: "10.0.0.0", want: true},
		{name: "prefix with host bits set", prefix: "10.0.5.1/24", ip: "10.0.5.200", want: true},
		{name: "IPv6 inside", prefix: "2001:db8::/32", ip: "2001:db8:1::1", want: true},
		{name: "IPv6 outside", prefix: "2001:db8::/32", ip: "2001:db9::1", want: false},
		{name: "v6 address against v4 prefix", prefix: "10.0.0.0/8", ip: "::1", want: false},
		{name: "v4 address against v6 prefix", prefix: "::/0", ip: "10.0.0.1", want: false},
		{name: "v4-mapped v6 address against v4 prefix", prefix: "10.0.0.0/8", ip: "::ffff:10.1.2.3", want: false},
		{name: "malformed prefix", prefix: "banana", ip: "10.0.0.1", wantErr: "no '/'"},
		{name: "malformed ip", prefix: "10.0.0.0/8", ip: "banana", wantErr: "unable to parse IP"},
		{name: "empty ip", prefix: "10.0.0.0/8", ip: "", wantErr: "unable to parse IP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Contains(tt.prefix, tt.ip)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Contains(%q, %q) error = %v, want it to contain %q", tt.prefix, tt.ip, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Contains(%q, %q) error = %v", tt.prefix, tt.ip, err)
			}
			if got != tt.want {
				t.Errorf("Contains(%q, %q) = %v, want %v", tt.prefix, tt.ip, got, tt.want)
			}
		})
	}
}

func TestOverlaps(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		want    bool
		wantErr string
	}{
		{name: "adjacent do not overlap", a: "10.0.0.0/25", b: "10.0.0.128/25", want: false},
		{name: "nested overlap", a: "10.0.0.0/16", b: "10.0.1.0/24", want: true},
		{name: "nested overlap reversed", a: "10.0.1.0/24", b: "10.0.0.0/16", want: true},
		{name: "identical overlap", a: "10.0.0.0/24", b: "10.0.0.0/24", want: true},
		{name: "disjoint", a: "10.0.0.0/8", b: "192.168.0.0/16", want: false},
		{name: "host bits masked before comparing", a: "10.0.0.129/25", b: "10.0.0.130/25", want: true},
		{name: "IPv6 nested", a: "2001:db8::/32", b: "2001:db8:ff::/48", want: true},
		{name: "IPv6 adjacent", a: "2001:db8::/33", b: "2001:db8:8000::/33", want: false},
		{name: "mixed families never overlap", a: "0.0.0.0/0", b: "::/0", want: false},
		{name: "malformed first prefix", a: "nope", b: "10.0.0.0/8", wantErr: "first prefix"},
		{name: "malformed second prefix", a: "10.0.0.0/8", b: "nope", wantErr: "second prefix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Overlaps(tt.a, tt.b)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Overlaps(%q, %q) error = %v, want it to contain %q", tt.a, tt.b, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Overlaps(%q, %q) error = %v", tt.a, tt.b, err)
			}
			if got != tt.want {
				t.Errorf("Overlaps(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		newLen  int
		want    []string
		wantLen int
		wantErr string
	}{
		{
			name: "IPv4 /24 into /26", prefix: "10.0.0.0/24", newLen: 26,
			want: []string{"10.0.0.0/26", "10.0.0.64/26", "10.0.0.128/26", "10.0.0.192/26"},
		},
		{
			name: "same length is identity", prefix: "10.0.0.0/24", newLen: 24,
			want: []string{"10.0.0.0/24"},
		},
		{
			name: "host bits masked before splitting", prefix: "10.0.0.77/24", newLen: 25,
			want: []string{"10.0.0.0/25", "10.0.0.128/25"},
		},
		{
			name: "IPv4 down to /32", prefix: "192.168.1.0/30", newLen: 32,
			want: []string{"192.168.1.0/32", "192.168.1.1/32", "192.168.1.2/32", "192.168.1.3/32"},
		},
		{
			name: "IPv6 /126 into /128", prefix: "2001:db8::/126", newLen: 128,
			want: []string{"2001:db8::/128", "2001:db8::1/128", "2001:db8::2/128", "2001:db8::3/128"},
		},
		{
			name: "IPv6 /64 into /66", prefix: "2001:db8::/64", newLen: 66,
			want: []string{"2001:db8::/66", "2001:db8:0:0:4000::/66", "2001:db8:0:0:8000::/66", "2001:db8:0:0:c000::/66"},
		},
		{name: "exactly 65536 subnets is allowed", prefix: "10.0.0.0/8", newLen: 24, wantLen: 65536},
		{name: "65537+ subnets refused with count", prefix: "10.0.0.0/8", newLen: 25, wantErr: "would produce 131072 subnets; refusing to print more than 65536"},
		{name: "IPv6 refusal reports exact big count", prefix: "::/0", newLen: 64, wantErr: "would produce 18446744073709551616 subnets"},
		{name: "new length shorter than prefix", prefix: "10.0.0.0/24", newLen: 16, wantErr: "must be between 24 (the prefix's own length) and 32"},
		{name: "new length beyond IPv4 width", prefix: "10.0.0.0/24", newLen: 33, wantErr: "must be between 24 (the prefix's own length) and 32"},
		{name: "new length beyond IPv6 width", prefix: "2001:db8::/64", newLen: 129, wantErr: "must be between 64 (the prefix's own length) and 128"},
		{name: "negative new length", prefix: "10.0.0.0/24", newLen: -1, wantErr: "must be between 24 (the prefix's own length) and 32"},
		{name: "malformed prefix", prefix: "banana", newLen: 24, wantErr: "no '/'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Split(tt.prefix, tt.newLen)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Split(%q, %d) error = %v, want it to contain %q", tt.prefix, tt.newLen, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Split(%q, %d) error = %v", tt.prefix, tt.newLen, err)
			}
			if tt.wantLen != 0 {
				if len(got) != tt.wantLen {
					t.Fatalf("Split(%q, %d) returned %d subnets, want %d", tt.prefix, tt.newLen, len(got), tt.wantLen)
				}
				if got[0] != "10.0.0.0/24" || got[len(got)-1] != "10.255.255.0/24" {
					t.Errorf("Split(%q, %d) endpoints = %q and %q, want 10.0.0.0/24 and 10.255.255.0/24", tt.prefix, tt.newLen, got[0], got[len(got)-1])
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Split(%q, %d) = %v, want %v", tt.prefix, tt.newLen, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Split(%q, %d)[%d] = %q, want %q", tt.prefix, tt.newLen, i, got[i], tt.want[i])
				}
			}
		})
	}
}
