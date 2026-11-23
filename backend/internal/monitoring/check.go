package monitoring

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"saturnita/pulse/internal/pulse"
	"strings"
	"time"
)

var denied = errors.New("blocked_target")
var oversized = errors.New("response_limit")
var special = []netip.Prefix{}

func init() {
	for _, s := range strings.Fields("0.0.0.0/8 100.64.0.0/10 192.0.0.0/24 192.0.2.0/24 192.31.196.0/24 192.52.193.0/24 192.88.99.0/24 192.175.48.0/24 198.18.0.0/15 198.51.100.0/24 203.0.113.0/24 240.0.0.0/4 168.63.129.16/32 2001::/23 2001:db8::/32 2002::/16 2620:4f:8000::/48 3fff::/20") {
		special = append(special, netip.MustParsePrefix(s))
	}
}

var global6 = netip.MustParsePrefix("2000::/3")

func public(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.Zone() != "" || a.Is6() && !global6.Contains(a) {
		return false
	}
	for _, p := range special {
		if p.Contains(a) {
			return false
		}
	}
	return true
}
func Target(raw string) (*url.URL, error) {
	u, ok := pulse.Target(raw)
	if !ok || strings.Contains(u.Host, "%") || strings.HasSuffix(u.Host, ":") {
		return nil, denied
	}
	h := pulse.Host(u.Hostname())
	if ip, e := netip.ParseAddr(h); e == nil {
		if !public(ip) {
			return nil, denied
		}
	} else if !pulse.ValidHost(h) || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".internal") {
		return nil, denied
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		return nil, denied
	}
	return u, nil
}

type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type Checker struct {
	lookup  resolver
	connect func(context.Context, string, string) (net.Conn, error)
}

func SafeChecker() Checker { d := net.Dialer{}; return Checker{net.DefaultResolver, d.DialContext} }

type Result struct {
	Code    *int
	Latency int
	Success bool
	Error   *string
}

func failure(kind string) Result { return Result{Error: &kind} }
func (c Checker) dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil || port != "80" && port != "443" {
		return nil, denied
	}
	var ips []netip.Addr
	if a, e := netip.ParseAddr(host); e == nil {
		ips = []netip.Addr{a}
	} else {
		ips, e = c.lookup.LookupNetIP(ctx, "ip", strings.TrimSuffix(host, ".")+".")
		if e != nil {
			return nil, e
		}
	}
	if len(ips) == 0 {
		return nil, denied
	}
	for _, a := range ips {
		if !public(a) {
			return nil, denied
		}
	}
	pinned := ips[0].Unmap()
	conn, e := c.connect(ctx, "tcp", net.JoinHostPort(pinned.String(), port))
	if e != nil {
		return nil, e
	}
	peer, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok || peer.AddrPort().Addr().Unmap() != pinned || !public(peer.AddrPort().Addr()) {
		conn.Close()
		return nil, denied
	}
	return conn, nil
}
func (c Checker) Check(parent context.Context, j Job) (r Result) {
	start := time.Now()
	defer func() { r.Latency = int(time.Since(start).Milliseconds()) }()
	u, e := Target(j.URL)
	if e != nil || j.Method != "GET" && j.Method != "HEAD" {
		return failure("blocked_target")
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(j.Timeout)*time.Millisecond)
	defer cancel()
	tr := &http.Transport{DialContext: c.dial, DisableKeepAlives: true, DisableCompression: true, MaxResponseHeaderBytes: 16384, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	defer tr.CloseIdleConnections()
	client := http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, j.Method, u.String(), nil)
	if e != nil {
		return failure("blocked_target")
	}
	req.Header.Set("User-Agent", "Saturnita-Pulse/1.0")
	resp, e := client.Do(req)
	if e == nil {
		defer resp.Body.Close()
		var n int64
		if j.Method == "GET" {
			n, e = io.Copy(io.Discard, io.LimitReader(resp.Body, 65537))
		}
		if n > 65536 {
			e = oversized
		}
		if e == nil {
			if resp.StatusCode >= j.Min && resp.StatusCode <= j.Max {
				return Result{Code: &resp.StatusCode, Success: true}
			}
			r = failure("http_status")
			r.Code = &resp.StatusCode
			return r
		}
	}
	var dns *net.DNSError
	var cert *tls.CertificateVerificationError
	var netErr net.Error
	switch {
	case errors.Is(e, denied):
		return failure("blocked_target")
	case errors.Is(e, oversized):
		return failure("response_limit")
	case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.As(e, &netErr) && netErr.Timeout():
		return failure("timeout")
	case errors.As(e, &dns):
		return failure("dns")
	case errors.As(e, &cert) || strings.Contains(e.Error(), "tls:"):
		return failure("tls")
	default:
		return failure("connect")
	}
}
