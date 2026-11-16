package utils

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP is the address a request came from. Straight from the internet
// it's the connection's own address. From a reverse proxy in front of the game
// (a loopback or private peer) it's the hop the proxy added to
// X-Forwarded-For: the last one, not the first, which the client writes
// itself. Trusting the first once let anyone claim to be 127.0.0.1 (passing
// the local-only checks) or a new address per request (beta exploit audit A6).
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil || !(peer.IsLoopback() || peer.IsPrivate()) {
		return host
	}
	var hops []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		for _, h := range strings.Split(v, ",") {
			if h = strings.TrimSpace(h); h != "" {
				hops = append(hops, h)
			}
		}
	}
	if len(hops) > 0 {
		return hops[len(hops)-1]
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}
	return host
}
