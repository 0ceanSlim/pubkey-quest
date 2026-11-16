package utils

import (
	"context"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ─── Public addresses only (beta exploit audit E4) ───────────────────────────
//
// The server connects to addresses players hand it: the relays in their own
// (or a friend's) relay lists, a portrait's URL. Left alone, a player could
// point one at the server's own network (localhost, 10.x, the cloud metadata
// address) and have the server knock on it for them. PublicHost turns away
// anything that isn't, or doesn't resolve only to, a public internet address.

var publicHosts = struct {
	sync.Mutex
	m map[string]publicEntry
}{m: map[string]publicEntry{}}

type publicEntry struct {
	ok bool
	at time.Time
}

const publicHostTTL = 10 * time.Minute

// PublicIP reports whether an address is on the public internet.
func PublicIP(ip net.IP) bool {
	return ip != nil && !(ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast())
}

// PublicHost reports whether a host name or address resolves only to public
// addresses (cached for a few minutes).
func PublicHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return PublicIP(ip)
	}
	publicHosts.Lock()
	e, ok := publicHosts.m[host]
	publicHosts.Unlock()
	if ok && time.Since(e.at) < publicHostTTL {
		return e.ok
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	public := err == nil && len(ips) > 0
	for _, a := range ips {
		if !PublicIP(a.IP) {
			public = false
		}
	}
	publicHosts.Lock()
	publicHosts.m[host] = publicEntry{ok: public, at: time.Now()}
	publicHosts.Unlock()
	return public
}

// PublicURL reports whether a URL uses one of the schemes and points at a
// public host.
func PublicURL(raw string, schemes ...string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	okScheme := false
	for _, s := range schemes {
		if strings.EqualFold(u.Scheme, s) {
			okScheme = true
		}
	}
	return okScheme && PublicHost(u.Hostname())
}

// PublicRelays keeps the relay URLs (wss://, or ws://) that point at public hosts.
func PublicRelays(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if PublicURL(u, "wss", "ws") {
			out = append(out, u)
		}
	}
	return out
}
