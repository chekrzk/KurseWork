package monitoring

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type answers []netip.Addr

func (a answers) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) { return a, nil }

type peer struct {
	net.Conn
	addr *net.TCPAddr
}

func (p peer) RemoteAddr() net.Addr { return p.addr }
func TestNetworkPolicy(t *testing.T) {
	for _, s := range strings.Fields("127.0.0.1 169.254.169.254 10.0.0.1 100.64.0.1 198.18.0.1 ::1 fc00::1 2001:db8::1 ::ffff:127.0.0.1") {
		if public(netip.MustParseAddr(s)) {
			t.Fatal(s)
		}
	}
	for _, raw := range []string{"ftp://example.org", "http://example.org:8080", "https://a:b@example.org/", "http://private.internal", "http://127.0.0.1"} {
		if _, e := Target(raw); e == nil {
			t.Fatal(raw)
		}
	}
	c := Checker{answers{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("dialed rejected answer")
		return nil, nil
	}}
	if _, e := c.dial(context.Background(), "tcp", "example.org:80"); e != denied {
		t.Fatal(e)
	}
	a, b := net.Pipe()
	defer b.Close()
	c.lookup = answers{netip.MustParseAddr("8.8.8.8")}
	c.connect = func(context.Context, string, string) (net.Conn, error) {
		return peer{a, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80}}, nil
	}
	if _, e := c.dial(context.Background(), "tcp", "example.org:80"); e != denied {
		t.Fatal("peer mismatch accepted")
	}
}
func TestCheckLimits(t *testing.T) {
	for _, tc := range []struct {
		name              string
		code, size, delay int
		want              string
	}{{"redirect", 302, 0, 0, ""}, {"status", 503, 0, 0, "http_status"}, {"large", 200, 65537, 0, "response_limit"}, {"slow", 200, 0, 150, "timeout"}} {
		t.Run(tc.name, func(t *testing.T) {
			var calls int
			c := Checker{lookup: answers{netip.MustParseAddr("8.8.8.8")}, connect: func(ctx context.Context, network, address string) (net.Conn, error) {
				calls++
				if address != "8.8.8.8:80" {
					t.Errorf("unbound dial %s", address)
				}
				a, b := net.Pipe()
				go func() {
					defer b.Close()
					req, e := http.ReadRequest(bufio.NewReader(b))
					if e != nil {
						return
					}
					if req.Host != "example.org" {
						t.Errorf("wrong host %s", req.Host)
					}
					time.Sleep(time.Duration(tc.delay) * time.Millisecond)
					fmt.Fprintf(b, "HTTP/1.1 %d Status\r\nLocation: http://localhost/\r\nContent-Length: %d\r\n\r\n", tc.code, tc.size)
					io.WriteString(b, strings.Repeat("a", tc.size))
				}()
				return peer{a, &net.TCPAddr{IP: net.ParseIP("8.8.8.8"), Port: 80}}, nil
			}}
			r := c.Check(context.Background(), Job{URL: "http://example.org/test", Method: "GET", Min: 200, Max: 399, Timeout: 100})
			if calls != 1 || tc.want == "" && !r.Success || tc.want != "" && (r.Error == nil || *r.Error != tc.want) {
				t.Fatalf("%+v calls=%d", r, calls)
			}
		})
	}
}
