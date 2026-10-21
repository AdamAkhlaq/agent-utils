// Command proxy is the benchmark's only egress: an HTTP CONNECT proxy that
// tunnels to an allowlist of hosts and refuses everything else. Trial
// containers sit on an internal Docker network with this proxy as their
// HTTPS_PROXY, so the only traffic that can leave is to the Claude API, and
// every refused host is logged as evidence.
//
// It is deliberately minimal: CONNECT only (no plain HTTP forwarding, which
// the agent never needs), one goroutine pair per tunnel, a cap on
// simultaneous tunnels, and a read deadline on the request line so an idle
// client cannot hold a slot forever.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	headerTimeout = 10 * time.Second
	dialTimeout   = 10 * time.Second
	// idleTimeout closes a tunnel neither side has used for this long, so a
	// client that opens connections and abandons them cannot hold its slot
	// forever. The API's longest streaming pause is far shorter.
	idleTimeout = 5 * time.Minute
	// maxHeaderBytes caps the CONNECT request line and headers. A client can
	// send unbounded bytes before the blank line otherwise, and ReadRequest
	// would buffer them all.
	maxHeaderBytes = 16 << 10
	maxTunnels     = 64
	maxLoggedBytes = 256
)

func main() {
	listen := flag.String("listen", ":3128", "address to listen on")
	allow := flag.String("allow", "api.anthropic.com:443", "comma-separated host:port pairs that may be reached")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "proxy: unexpected argument %q\n", flag.Arg(0))
		os.Exit(2)
	}
	allowed, err := parseAllowlist(*allow)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proxy: %v\n", err)
		os.Exit(2)
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proxy: %v\n", err)
		os.Exit(1)
	}
	log.SetFlags(log.LstdFlags | log.LUTC)
	log.Printf("listening on %s, allowing %s", ln.Addr(), strings.Join(sortedHosts(allowed), ", "))
	serve(ln, allowed)
}

// parseAllowlist reads comma-separated host:port pairs. Each entry must name
// a port, so a typo cannot widen the allowlist to every port of a host.
func parseAllowlist(list string) (map[string]bool, error) {
	allowed := map[string]bool{}
	for _, entry := range strings.Split(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(entry); err != nil {
			return nil, fmt.Errorf("-allow %q: want host:port", entry)
		}
		allowed[entry] = true
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("-allow: at least one host:port is required")
	}
	return allowed, nil
}

func sortedHosts(set map[string]bool) []string {
	hosts := make([]string, 0, len(set))
	for h := range set {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts
}

// serve accepts connections until the listener closes. The semaphore bounds
// the number of live tunnels; a client past the cap waits in the accept
// queue rather than being refused, since trials are few and short.
func serve(ln net.Listener, allowed map[string]bool) {
	slots := make(chan struct{}, maxTunnels)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		slots <- struct{}{}
		go func() {
			defer func() { <-slots }()
			handle(conn, allowed)
		}()
	}
}

// handle serves one client connection: exactly one CONNECT, then a tunnel
// until both directions are done or the tunnel goes idle.
func handle(client net.Conn, allowed map[string]bool) {
	defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(headerTimeout))
	// The header cap is lifted once the request is read, and the tunnel
	// copies from the same buffered reader, so bytes a client sends in the
	// same segment as its headers reach the upstream instead of being lost.
	limited := &io.LimitedReader{R: client, N: maxHeaderBytes}
	br := bufio.NewReader(limited)
	req, err := http.ReadRequest(br)
	if err != nil {
		if limited.N == 0 {
			log.Printf("refused request from %s: headers exceed %d bytes", client.RemoteAddr(), maxHeaderBytes)
			reply(client, http.StatusRequestHeaderFieldsTooLarge)
		}
		return
	}
	if req.Method != http.MethodConnect {
		log.Printf("refused %s %s from %s: only CONNECT is served", req.Method, loggable(req.URL.Path), client.RemoteAddr())
		reply(client, http.StatusMethodNotAllowed)
		return
	}
	if !allowed[req.Host] {
		log.Printf("refused CONNECT %s from %s", loggable(req.Host), client.RemoteAddr())
		reply(client, http.StatusForbidden)
		return
	}
	upstream, err := net.DialTimeout("tcp", req.Host, dialTimeout)
	if err != nil {
		log.Printf("dial %s: %v", req.Host, err)
		reply(client, http.StatusBadGateway)
		return
	}
	defer upstream.Close()
	limited.N = math.MaxInt64
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	tunnel(client, br, upstream)
}

// tunnel copies both directions until each has finished. When one side
// stops sending, the other side's write end is closed so it sees EOF, and
// its pending response still flows the other way. Every byte copied pushes
// both deadlines out by idleTimeout; a tunnel with no traffic in that long
// is closed by the deadline firing in whichever copy is blocked.
func tunnel(client net.Conn, clientReader io.Reader, upstream net.Conn) {
	touch := func() {
		deadline := time.Now().Add(idleTimeout)
		_ = client.SetDeadline(deadline)
		_ = upstream.SetDeadline(deadline)
	}
	touch()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(touching{upstream, touch}, clientReader)
		closeWrite(upstream)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(touching{client, touch}, upstream)
		closeWrite(client)
	}()
	wg.Wait()
}

// touching resets the idle deadline on every write.
type touching struct {
	w     io.Writer
	touch func()
}

func (t touching) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	t.touch()
	return n, err
}

func closeWrite(conn net.Conn) {
	if cw, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = conn.Close()
}

// loggable bounds and quotes what a client sent, so a hostile target cannot
// grow the log or forge lines in it.
func loggable(s string) string {
	if len(s) > maxLoggedBytes {
		return fmt.Sprintf("%q [+%d bytes]", s[:maxLoggedBytes], len(s)-maxLoggedBytes)
	}
	return fmt.Sprintf("%q", s)
}

func reply(conn net.Conn, status int) {
	_, _ = fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", status, http.StatusText(status))
}
