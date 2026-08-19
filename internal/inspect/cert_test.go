package inspect

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Fixture certs are generated per test run with fixed validity windows and a
// fixed "now", so results are deterministic and nothing ages out.
var (
	fixtureNotBefore = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	fixtureNotAfter  = time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	fixtureNow       = time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
)

func pemEncode(t *testing.T, blockType string, der []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatalf("encoding PEM: %v", err)
	}
	return buf.Bytes()
}

func selfSignedDER(t *testing.T, template *x509.Certificate, pub, priv any) []byte {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	if err != nil {
		t.Fatalf("creating certificate: %v", err)
	}
	return der
}

func rsaSelfSignedPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1000),
		Subject:               pkix.Name{CommonName: "rsa.example.com", Organization: []string{"Example Org"}},
		NotBefore:             fixtureNotBefore,
		NotAfter:              fixtureNotAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	return pemEncode(t, "CERTIFICATE", selfSignedDER(t, template, &key.PublicKey, key))
}

func ecdsaSANsDER(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating ECDSA key: %v", err)
	}
	uri, err := url.Parse("spiffe://cluster/workload")
	if err != nil {
		t.Fatalf("parsing URI: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:   big.NewInt(2000),
		Subject:        pkix.Name{CommonName: "ecdsa.example.com"},
		NotBefore:      fixtureNotBefore,
		NotAfter:       fixtureNotAfter,
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:       []string{"ecdsa.example.com", "www.ecdsa.example.com"},
		IPAddresses:    []net.IP{net.ParseIP("192.0.2.10"), net.ParseIP("2001:db8::1")},
		EmailAddresses: []string{"admin@example.com"},
		URIs:           []*url.URL{uri},
	}
	return selfSignedDER(t, template, &key.PublicKey, key)
}

func ed25519ExpiredPEM(t *testing.T) []byte {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating Ed25519 key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(3000),
		Subject:      pkix.Name{CommonName: "expired.example.com"},
		NotBefore:    fixtureNotBefore,
		NotAfter:     time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	return pemEncode(t, "CERTIFICATE", selfSignedDER(t, template, pub, priv))
}

// chainPEM returns a two-entry PEM: a leaf signed by a CA, then the CA.
func chainPEM(t *testing.T) []byte {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generating CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(4000),
		Subject:               pkix.Name{CommonName: "Test Root CA"},
		NotBefore:             fixtureNotBefore,
		NotAfter:              fixtureNotAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caDER := selfSignedDER(t, caTemplate, &caKey.PublicKey, caKey)
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parsing CA certificate: %v", err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating leaf key: %v", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(4001),
		Subject:      pkix.Name{CommonName: "leaf.example.com"},
		NotBefore:    fixtureNotBefore,
		NotAfter:     fixtureNotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"leaf.example.com"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("creating leaf certificate: %v", err)
	}
	return append(pemEncode(t, "CERTIFICATE", leafDER), pemEncode(t, "CERTIFICATE", caDER)...)
}

func privateKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshaling private key: %v", err)
	}
	return pemEncode(t, "PRIVATE KEY", der)
}

func TestCertificates(t *testing.T) {
	tests := []struct {
		name  string
		input func(t *testing.T) []byte
		check func(t *testing.T, certs []Certificate)
	}{
		{
			name:  "self-signed rsa pem",
			input: rsaSelfSignedPEM,
			check: func(t *testing.T, certs []Certificate) {
				if len(certs) != 1 {
					t.Fatalf("got %d certificates, want 1", len(certs))
				}
				c := certs[0]
				if want := "CN=rsa.example.com,O=Example Org"; c.Subject != want {
					t.Errorf("Subject = %q, want %q", c.Subject, want)
				}
				if c.Subject != c.Issuer {
					t.Errorf("Issuer = %q, want it to equal the subject", c.Issuer)
				}
				if !c.SelfSigned {
					t.Error("SelfSigned = false, want true")
				}
				if c.Expired {
					t.Error("Expired = true, want false")
				}
				if c.SerialNumber != "1000" {
					t.Errorf("SerialNumber = %q, want %q", c.SerialNumber, "1000")
				}
				if c.NotBefore != "2020-01-01T00:00:00Z" || c.NotAfter != "2040-01-01T00:00:00Z" {
					t.Errorf("validity = %q to %q, want the fixture window", c.NotBefore, c.NotAfter)
				}
				if c.PublicKeyAlgorithm != "RSA" || c.PublicKeyBits != 2048 {
					t.Errorf("public key = %s/%d, want RSA/2048", c.PublicKeyAlgorithm, c.PublicKeyBits)
				}
				if c.Curve != "" {
					t.Errorf("Curve = %q, want empty for RSA", c.Curve)
				}
				wantKU := []string{"digitalSignature", "keyEncipherment"}
				if !equalStrings(c.KeyUsage, wantKU) {
					t.Errorf("KeyUsage = %v, want %v", c.KeyUsage, wantKU)
				}
				if !equalStrings(c.ExtKeyUsage, []string{"serverAuth"}) {
					t.Errorf("ExtKeyUsage = %v, want [serverAuth]", c.ExtKeyUsage)
				}
				if c.IsCA {
					t.Error("IsCA = true, want false")
				}
				if len(c.DNSNames) != 0 || len(c.IPAddresses) != 0 || len(c.EmailAddresses) != 0 || len(c.URIs) != 0 {
					t.Errorf("SANs not empty: %v %v %v %v", c.DNSNames, c.IPAddresses, c.EmailAddresses, c.URIs)
				}
			},
		},
		{
			name:  "ecdsa der with every san type",
			input: ecdsaSANsDER,
			check: func(t *testing.T, certs []Certificate) {
				if len(certs) != 1 {
					t.Fatalf("got %d certificates, want 1", len(certs))
				}
				c := certs[0]
				if !equalStrings(c.DNSNames, []string{"ecdsa.example.com", "www.ecdsa.example.com"}) {
					t.Errorf("DNSNames = %v", c.DNSNames)
				}
				if !equalStrings(c.IPAddresses, []string{"192.0.2.10", "2001:db8::1"}) {
					t.Errorf("IPAddresses = %v", c.IPAddresses)
				}
				if !equalStrings(c.EmailAddresses, []string{"admin@example.com"}) {
					t.Errorf("EmailAddresses = %v", c.EmailAddresses)
				}
				if !equalStrings(c.URIs, []string{"spiffe://cluster/workload"}) {
					t.Errorf("URIs = %v", c.URIs)
				}
				if c.PublicKeyAlgorithm != "ECDSA" || c.PublicKeyBits != 256 || c.Curve != "P-256" {
					t.Errorf("public key = %s/%d/%s, want ECDSA/256/P-256",
						c.PublicKeyAlgorithm, c.PublicKeyBits, c.Curve)
				}
				if !equalStrings(c.ExtKeyUsage, []string{"serverAuth", "clientAuth"}) {
					t.Errorf("ExtKeyUsage = %v", c.ExtKeyUsage)
				}
				if !c.SelfSigned {
					t.Error("SelfSigned = false, want true")
				}
			},
		},
		{
			name:  "expired ed25519 cert still decodes",
			input: ed25519ExpiredPEM,
			check: func(t *testing.T, certs []Certificate) {
				if len(certs) != 1 {
					t.Fatalf("got %d certificates, want 1", len(certs))
				}
				c := certs[0]
				if !c.Expired {
					t.Error("Expired = false, want true")
				}
				if c.NotAfter != "2021-01-01T00:00:00Z" {
					t.Errorf("NotAfter = %q, want 2021-01-01T00:00:00Z", c.NotAfter)
				}
				if c.PublicKeyAlgorithm != "Ed25519" || c.PublicKeyBits != 256 {
					t.Errorf("public key = %s/%d, want Ed25519/256", c.PublicKeyAlgorithm, c.PublicKeyBits)
				}
			},
		},
		{
			name:  "chain pem yields entries in order",
			input: chainPEM,
			check: func(t *testing.T, certs []Certificate) {
				if len(certs) != 2 {
					t.Fatalf("got %d certificates, want 2", len(certs))
				}
				leaf, ca := certs[0], certs[1]
				if leaf.Subject != "CN=leaf.example.com" {
					t.Errorf("leaf Subject = %q", leaf.Subject)
				}
				if leaf.Issuer != "CN=Test Root CA" {
					t.Errorf("leaf Issuer = %q", leaf.Issuer)
				}
				if leaf.SelfSigned {
					t.Error("leaf SelfSigned = true, want false")
				}
				if leaf.IsCA {
					t.Error("leaf IsCA = true, want false")
				}
				if !ca.SelfSigned {
					t.Error("CA SelfSigned = false, want true")
				}
				if !ca.IsCA {
					t.Error("CA IsCA = false, want true")
				}
				if !equalStrings(ca.KeyUsage, []string{"certSign", "crlSign"}) {
					t.Errorf("CA KeyUsage = %v, want [certSign crlSign]", ca.KeyUsage)
				}
				if ca.Curve != "P-384" || ca.PublicKeyBits != 384 {
					t.Errorf("CA key = %s/%d, want P-384/384", ca.Curve, ca.PublicKeyBits)
				}
			},
		},
		{
			name: "mixed pem with private key and certificate",
			input: func(t *testing.T) []byte {
				return append(privateKeyPEM(t), rsaSelfSignedPEM(t)...)
			},
			check: func(t *testing.T, certs []Certificate) {
				if len(certs) != 1 {
					t.Fatalf("got %d certificates, want 1", len(certs))
				}
				if certs[0].Subject != "CN=rsa.example.com,O=Example Org" {
					t.Errorf("Subject = %q", certs[0].Subject)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certs, err := Certificates(bytes.NewReader(tt.input(t)), fixtureNow)
			if err != nil {
				t.Fatalf("Certificates() error = %v", err)
			}
			tt.check(t, certs)
		})
	}
}

func TestCertificatesFingerprint(t *testing.T) {
	der := ecdsaSANsDER(t)
	sum := sha256.Sum256(der)
	want := hex.EncodeToString(sum[:])
	certs, err := Certificates(bytes.NewReader(der), fixtureNow)
	if err != nil {
		t.Fatalf("Certificates() error = %v", err)
	}
	if got := certs[0].SHA256Fingerprint; got != want {
		t.Errorf("SHA256Fingerprint = %q, want %q", got, want)
	}
	if strings.ContainsAny(want, ":ABCDEF") {
		t.Errorf("fingerprint %q is not lowercase colon-free hex", want)
	}
}

func TestCertificatesErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       func(t *testing.T) []byte
		wantMention []string
	}{
		{
			name:        "empty input",
			input:       func(t *testing.T) []byte { return nil },
			wantMention: []string{"empty input"},
		},
		{
			name:        "whitespace only",
			input:       func(t *testing.T) []byte { return []byte("  \n\t") },
			wantMention: []string{"empty input"},
		},
		{
			name:        "garbage input",
			input:       func(t *testing.T) []byte { return []byte("definitely not a certificate") },
			wantMention: []string{"neither PEM nor a DER certificate"},
		},
		{
			name:        "pem with only a private key",
			input:       privateKeyPEM,
			wantMention: []string{"no CERTIFICATE block", "PRIVATE KEY"},
		},
		{
			name: "pem certificate block with corrupt der",
			input: func(t *testing.T) []byte {
				return pemEncode(t, "CERTIFICATE", []byte("corrupt"))
			},
			wantMention: []string{"parsing PEM certificate 1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Certificates(bytes.NewReader(tt.input(t)), fixtureNow)
			if err == nil {
				t.Fatal("Certificates() expected an error, got nil")
			}
			for _, want := range tt.wantMention {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Certificates() error = %q, want it to mention %q", err, want)
				}
			}
		})
	}
}

func TestCertJSON(t *testing.T) {
	certs, err := Certificates(bytes.NewReader(rsaSelfSignedPEM(t)), fixtureNow)
	if err != nil {
		t.Fatalf("Certificates() error = %v", err)
	}
	out, err := CertJSON(certs)
	if err != nil {
		t.Fatalf("CertJSON() error = %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("CertJSON() output is not a JSON array: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("decoded %d entries, want 1", len(decoded))
	}
	for _, field := range []string{"dnsNames", "ipAddresses", "emailAddresses", "uris", "keyUsage", "extKeyUsage"} {
		v, ok := decoded[0][field]
		if !ok {
			t.Errorf("field %q missing from JSON output", field)
			continue
		}
		if _, isArray := v.([]any); !isArray {
			t.Errorf("field %q = %v, want a JSON array (never null)", field, v)
		}
	}
	if !strings.HasPrefix(out, "[") {
		t.Errorf("CertJSON() output does not start with an array bracket: %q", out[:1])
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
