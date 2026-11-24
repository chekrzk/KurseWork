package pulse

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var label = regexp.MustCompile("^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$")
var slugRule = regexp.MustCompile("^[a-z0-9][a-z0-9_-]{0,99}$")
var reserved = map[string]bool{"www": true, "api": true, "go": true, "admin": true, "mail": true, "login": true, "register": true, "dashboard": true, "health": true, "healthz": true, "readyz": true, "app": true, "internal": true, "assets": true, "collect": true, "localhost": true}

func Host(s string) string { return strings.TrimSuffix(strings.ToLower(s), ".") }
func ValidHost(s string) bool {
	if len(s) > 253 || !strings.Contains(s, ".") {
		return false
	}
	if _, e := netip.ParseAddr(s); e == nil {
		return false
	}
	for _, p := range strings.Split(s, ".") {
		if !label.MatchString(p) {
			return false
		}
	}
	return true
}
func Label(s string) bool { return label.MatchString(s) && !reserved[s] }
func Slug(s string) bool  { return slugRule.MatchString(s) && !reserved[s] }
func Token(n int) string  { b := make([]byte, n); rand.Read(b); return hex.EncodeToString(b) }
func Target(raw string) (*url.URL, bool) {
	u, e := url.Parse(raw)
	if e != nil || len(raw) > 2048 || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || strings.ContainsAny(raw, "\r\n\\") {
		return nil, false
	}
	return u, true
}
func ValidUTM(m map[string]string) bool {
	for k, v := range m {
		switch k {
		case "source", "medium", "campaign", "content", "term":
		default:
			return false
		}
		if len(v) > 255 {
			return false
		}
	}
	return true
}

// Keep unrelated raw query pairs byte-for-byte, including repeated keys.
func Location(raw string, utm map[string]string) string {
	if len(utm) == 0 {
		return raw
	}
	u, _ := url.Parse(raw)
	parts := []string{}
	if u.RawQuery != "" {
		for _, p := range strings.Split(u.RawQuery, "&") {
			k, _, _ := strings.Cut(p, "=")
			key, _ := url.QueryUnescape(k)
			if strings.HasPrefix(key, "utm_") {
				if _, ok := utm[strings.TrimPrefix(key, "utm_")]; ok {
					continue
				}
			}
			parts = append(parts, p)
		}
	}
	keys := []string{}
	for k := range utm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, "utm_"+k+"="+url.QueryEscape(utm[k]))
	}
	u.RawQuery = strings.Join(parts, "&")
	return u.String()
}
func mac(key []byte, fields ...string) []byte {
	h := hmac.New(sha256.New, key)
	for _, f := range fields {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(f)))
		h.Write(size[:])
		h.Write([]byte(f))
	}
	return h.Sum(nil)
}
func Visitor(master []byte, t time.Time, scope, ip, ua string) []byte {
	addr, e := netip.ParseAddr(ip)
	if e == nil {
		ip = addr.Unmap().String()
	} else {
		ip = ""
	}
	if len(ua) > 512 {
		ua = ua[:512]
	}
	return mac(mac(master, "v1", t.UTC().Format("2006-01-02"), scope), ip, strings.Join(strings.Fields(ua), " "))
}
func Agent(ua string) (device, browser, os string, bot bool) {
	ua = strings.ToLower(ua)
	device, browser, os = "desktop", "other", "other"
	switch {
	case strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet"):
		device = "tablet"
	case strings.Contains(ua, "mobile") || strings.Contains(ua, "android"):
		device = "mobile"
	}
	for _, p := range [][2]string{{"edg/", "Edge"}, {"firefox/", "Firefox"}, {"chrome/", "Chrome"}, {"safari/", "Safari"}} {
		if strings.Contains(ua, p[0]) {
			browser = p[1]
			break
		}
	}
	for _, p := range [][2]string{{"android", "Android"}, {"iphone", "iOS"}, {"ipad", "iOS"}, {"windows", "Windows"}, {"mac os", "macOS"}, {"linux", "Linux"}} {
		if strings.Contains(ua, p[0]) {
			os = p[1]
			break
		}
	}
	bot = strings.Contains(ua, "bot") || strings.Contains(ua, "crawler") || strings.Contains(ua, "spider")
	return
}
func Ref(raw string) string {
	u, ok := Target(raw)
	if ok {
		return Host(u.Hostname())
	}
	return ""
}
