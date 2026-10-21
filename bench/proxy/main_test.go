package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"
)

// startProxy serves on a loopback listener and returns its address.
func startProxy(t *testing.T, allowed map[string]bool) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go serve(ln, allowed)
	return ln.Addr().String()
}

// echoServer accepts one connection and echoes what it reads, standing in
// for the upstream a tunnel reaches.
func echoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return ln.Addr().String()
}

func connect(t *testing.T, proxy, target string) (net.Conn, *http.Response) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", proxy, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := io.WriteString(conn, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn, resp
}

func TestTunnelToAllowedHost(t *testing.T) {
	upstream := echoServer(t)
	proxy := startProxy(t, map[string]bool{upstream: true})
	conn, resp := connect(t, proxy, upstream)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d, want 200", resp.StatusCode)
	}
	if _, err := io.WriteString(conn, "ping"); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("echo = %q, %v; want ping", buf, err)
	}
}

func TestRefusals(t *testing.T) {
	upstream := echoServer(t)
	proxy := startProxy(t, map[string]bool{"api.anthropic.com:443": true})
	tests := []struct {
		name   string
		target string
		want   int
	}{
		{"host not on the allowlist", upstream, http.StatusForbidden},
		{"allowed host on another port", "api.anthropic.com:80", http.StatusForbidden},
		{"no port", "api.anthropic.com", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, resp := connect(t, proxy, tt.target)
			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
}

func TestPlainRequestRefused(t *testing.T) {
	proxy := startProxy(t, map[string]bool{"api.anthropic.com:443": true})
	conn, err := net.DialTimeout("tcp", proxy, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET http://example.com/ HTTP/1.1\r\nHost: example.com\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestGarbageIsDropped(t *testing.T) {
	proxy := startProxy(t, map[string]bool{"api.anthropic.com:443": true})
	conn, err := net.DialTimeout("tcp", proxy, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "\x16\x03\x01 not http\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadAll(conn); err != nil {
		t.Fatalf("expected the connection to be closed, got %v", err)
	}
}

func TestParseAllowlist(t *testing.T) {
	tests := []struct {
		list    string
		want    string
		wantErr bool
	}{
		{"api.anthropic.com:443", "api.anthropic.com:443", false},
		{" a.example:443 , b.example:8443 ", "a.example:443 b.example:8443", false},
		{"api.anthropic.com", "", true},
		{"", "", true},
		{",", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.list, func(t *testing.T) {
			got, err := parseAllowlist(tt.list)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if s := strings.Join(sortedHosts(got), " "); s != tt.want {
				t.Errorf("allowlist = %q, want %q", s, tt.want)
			}
		})
	}
}

func TestHeaderCap(t *testing.T) {
	proxy := startProxy(t, map[string]bool{"api.anthropic.com:443": true})
	conn, err := net.DialTimeout("tcp", proxy, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// A request line far past the cap, sent without the terminating blank
	// line: the proxy must answer before reading it all.
	if _, err := io.WriteString(conn, "CONNECT "+strings.Repeat("a", 4*maxHeaderBytes)); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("no response within the deadline: %v", err)
	}
	if resp.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Errorf("status = %d, want 431", resp.StatusCode)
	}
}

func TestPipelinedBytesReachUpstream(t *testing.T) {
	upstream := echoServer(t)
	proxy := startProxy(t, map[string]bool{upstream: true})
	conn, err := net.DialTimeout("tcp", proxy, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Headers and the first payload bytes in one write, as an optimistic
	// client does; the bytes must not be lost to the header buffer.
	if _, err := io.WriteString(conn, "CONNECT "+upstream+" HTTP/1.1\r\nHost: "+upstream+"\r\n\r\nping"); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT: %v, %v", resp, err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(br, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("echo = %q, %v; want the pipelined ping", buf, err)
	}
}

func TestHalfCloseDeliversResponse(t *testing.T) {
	// An upstream that answers only after the client has finished sending.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.ReadAll(conn) // until the client's FIN
		_, _ = io.WriteString(conn, "late reply")
	}()
	proxy := startProxy(t, map[string]bool{ln.Addr().String(): true})
	conn, resp := connect(t, proxy, ln.Addr().String())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d", resp.StatusCode)
	}
	if _, err := io.WriteString(conn, "request"); err != nil {
		t.Fatal(err)
	}
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got, err := io.ReadAll(conn)
	if err != nil || string(got) != "late reply" {
		t.Fatalf("response after half-close = %q, %v; want \"late reply\"", got, err)
	}
}

func TestTunnelTeardown(t *testing.T) {
	upstream := echoServer(t)
	proxy := startProxy(t, map[string]bool{upstream: true})
	before := runtime.NumGoroutine()
	conn, resp := connect(t, proxy, upstream)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d", resp.StatusCode)
	}
	conn.Close()
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Errorf("%d goroutines after the tunnel closed, %d before", n, before)
	}
}

func TestLoggable(t *testing.T) {
	if got := loggable("api.anthropic.com:443"); got != `"api.anthropic.com:443"` {
		t.Errorf("loggable = %s", got)
	}
	long := strings.Repeat("x", maxLoggedBytes+10)
	got := loggable(long)
	if !strings.HasSuffix(got, ` [+10 bytes]`) || len(got) > maxLoggedBytes+20 {
		t.Errorf("loggable(long) = %q", got)
	}
	if got := loggable("a\nb"); got != `"a\nb"` {
		t.Errorf("control characters not escaped: %s", got)
	}
}
