package inspect

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"strings"
	"time"
)

// Certificate is one decoded X.509 certificate, shaped for JSON output. The
// SAN arrays, KeyUsage, and ExtKeyUsage are always non-nil so they marshal as
// [] rather than null, keeping the output shape stable for consumers.
type Certificate struct {
	Subject            string   `json:"subject"`
	Issuer             string   `json:"issuer"`
	SerialNumber       string   `json:"serialNumber"`
	NotBefore          string   `json:"notBefore"`
	NotAfter           string   `json:"notAfter"`
	Expired            bool     `json:"expired"`
	SelfSigned         bool     `json:"selfSigned"`
	DNSNames           []string `json:"dnsNames"`
	IPAddresses        []string `json:"ipAddresses"`
	EmailAddresses     []string `json:"emailAddresses"`
	URIs               []string `json:"uris"`
	IsCA               bool     `json:"isCA"`
	KeyUsage           []string `json:"keyUsage"`
	ExtKeyUsage        []string `json:"extKeyUsage"`
	SignatureAlgorithm string   `json:"signatureAlgorithm"`
	PublicKeyAlgorithm string   `json:"publicKeyAlgorithm"`
	PublicKeyBits      int      `json:"publicKeyBits,omitempty"`
	Curve              string   `json:"curve,omitempty"`
	SHA256Fingerprint  string   `json:"sha256Fingerprint"`
}

// Certificates decodes every X.509 certificate in r. PEM input yields one
// entry per CERTIFICATE block in order; input without PEM markers is parsed
// as a single DER certificate. Expired certificates decode successfully:
// decoding is inspection, not validation. now feeds the Expired field, so
// callers control the clock.
func Certificates(r io.Reader, now time.Time) ([]Certificate, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	parsed, err := parseCertificates(data)
	if err != nil {
		return nil, err
	}
	out := make([]Certificate, len(parsed))
	for i, c := range parsed {
		out[i] = describeCertificate(c, now)
	}
	return out, nil
}

// CertJSON renders certs as a pretty-printed JSON array. The result is always
// an array, even for a single certificate, so the shape is stable.
func CertJSON(certs []Certificate) (string, error) {
	out, err := json.MarshalIndent(certs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding certificates: %w", err)
	}
	return string(out), nil
}

func parseCertificates(data []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	var otherTypes []string
	sawPEM := false
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		sawPEM = true
		if block.Type != "CERTIFICATE" {
			otherTypes = append(otherTypes, block.Type)
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PEM certificate %d: %w", len(certs)+1, err)
		}
		certs = append(certs, c)
	}
	if sawPEM {
		if len(certs) == 0 {
			return nil, fmt.Errorf("input contains PEM blocks but no CERTIFICATE block (found: %s)",
				strings.Join(dedupe(otherTypes), ", "))
		}
		return certs, nil
	}
	c, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, fmt.Errorf("input is neither PEM nor a DER certificate: %w", err)
	}
	return []*x509.Certificate{c}, nil
}

func dedupe(items []string) []string {
	seen := make(map[string]bool, len(items))
	var out []string
	for _, s := range items {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func describeCertificate(c *x509.Certificate, now time.Time) Certificate {
	ips := make([]string, 0, len(c.IPAddresses))
	for _, ip := range c.IPAddresses {
		ips = append(ips, ip.String())
	}
	uris := make([]string, 0, len(c.URIs))
	for _, u := range c.URIs {
		uris = append(uris, u.String())
	}
	// CheckSignature (not CheckSignatureFrom) verifies against the cert's own
	// key without CA constraints, so a self-signed non-CA leaf still counts.
	selfSigned := bytes.Equal(c.RawSubject, c.RawIssuer) &&
		c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature) == nil
	bits, curve := publicKeySize(c)
	fingerprint := sha256.Sum256(c.Raw)
	return Certificate{
		Subject:            c.Subject.String(),
		Issuer:             c.Issuer.String(),
		SerialNumber:       c.SerialNumber.String(),
		NotBefore:          c.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:           c.NotAfter.UTC().Format(time.RFC3339),
		Expired:            now.After(c.NotAfter),
		SelfSigned:         selfSigned,
		DNSNames:           append([]string{}, c.DNSNames...),
		IPAddresses:        ips,
		EmailAddresses:     append([]string{}, c.EmailAddresses...),
		URIs:               uris,
		IsCA:               c.IsCA,
		KeyUsage:           keyUsageNames(c.KeyUsage),
		ExtKeyUsage:        extKeyUsageNames(c),
		SignatureAlgorithm: c.SignatureAlgorithm.String(),
		PublicKeyAlgorithm: c.PublicKeyAlgorithm.String(),
		PublicKeyBits:      bits,
		Curve:              curve,
		SHA256Fingerprint:  hex.EncodeToString(fingerprint[:]),
	}
}

func publicKeySize(c *x509.Certificate) (bits int, curve string) {
	switch key := c.PublicKey.(type) {
	case *rsa.PublicKey:
		return key.N.BitLen(), ""
	case *ecdsa.PublicKey:
		return key.Curve.Params().BitSize, key.Curve.Params().Name
	case ed25519.PublicKey:
		return ed25519.PublicKeySize * 8, ""
	default:
		return 0, ""
	}
}

func keyUsageNames(usage x509.KeyUsage) []string {
	table := []struct {
		bit  x509.KeyUsage
		name string
	}{
		{x509.KeyUsageDigitalSignature, "digitalSignature"},
		{x509.KeyUsageContentCommitment, "contentCommitment"},
		{x509.KeyUsageKeyEncipherment, "keyEncipherment"},
		{x509.KeyUsageDataEncipherment, "dataEncipherment"},
		{x509.KeyUsageKeyAgreement, "keyAgreement"},
		{x509.KeyUsageCertSign, "certSign"},
		{x509.KeyUsageCRLSign, "crlSign"},
		{x509.KeyUsageEncipherOnly, "encipherOnly"},
		{x509.KeyUsageDecipherOnly, "decipherOnly"},
	}
	names := []string{}
	for _, entry := range table {
		if usage&entry.bit != 0 {
			names = append(names, entry.name)
		}
	}
	return names
}

var extKeyUsageTable = map[x509.ExtKeyUsage]string{
	x509.ExtKeyUsageAny:                            "any",
	x509.ExtKeyUsageServerAuth:                     "serverAuth",
	x509.ExtKeyUsageClientAuth:                     "clientAuth",
	x509.ExtKeyUsageCodeSigning:                    "codeSigning",
	x509.ExtKeyUsageEmailProtection:                "emailProtection",
	x509.ExtKeyUsageIPSECEndSystem:                 "ipsecEndSystem",
	x509.ExtKeyUsageIPSECTunnel:                    "ipsecTunnel",
	x509.ExtKeyUsageIPSECUser:                      "ipsecUser",
	x509.ExtKeyUsageTimeStamping:                   "timeStamping",
	x509.ExtKeyUsageOCSPSigning:                    "ocspSigning",
	x509.ExtKeyUsageMicrosoftServerGatedCrypto:     "microsoftServerGatedCrypto",
	x509.ExtKeyUsageNetscapeServerGatedCrypto:      "netscapeServerGatedCrypto",
	x509.ExtKeyUsageMicrosoftCommercialCodeSigning: "microsoftCommercialCodeSigning",
	x509.ExtKeyUsageMicrosoftKernelCodeSigning:     "microsoftKernelCodeSigning",
}

func extKeyUsageNames(c *x509.Certificate) []string {
	names := []string{}
	for _, eku := range c.ExtKeyUsage {
		if name, ok := extKeyUsageTable[eku]; ok {
			names = append(names, name)
		} else {
			names = append(names, fmt.Sprintf("unknown(%d)", eku))
		}
	}
	for _, oid := range c.UnknownExtKeyUsage {
		names = append(names, oid.String())
	}
	return names
}
