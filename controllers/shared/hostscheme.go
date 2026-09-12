package shared

import (
	"net/url"
	"strings"
)

// InferHostScheme parses a FreqUI-style host value into a (scheme, hostname)
// pair: an explicit "http://" or "https://" prefix wins outright, a
// "localhost" prefix is always http (nobody terminates TLS on their own
// laptop), and anything else falls back to defaultScheme. Shared between
// controllers/tradebot's collectCORSHostsForTradeBot (default "https",
// its own long-standing behavior) and FreqUI's own status.url derivation
// (G4-1, GATEWAY-API-PLAN.md - default depends on whether TLS/a Gateway
// listener implies it) so the two don't drift into two subtly different
// copies of the same parsing.
func InferHostScheme(host, defaultScheme string) (scheme, hostname string) {
	scheme, hostname = defaultScheme, host

	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		if u, err := url.Parse(host); err == nil && u.Host != "" {
			return u.Scheme, u.Host
		}
		return scheme, hostname
	}
	if strings.HasPrefix(host, "localhost") {
		return "http", hostname
	}
	return scheme, hostname
}
