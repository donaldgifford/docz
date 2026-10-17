package oauth

import (
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
)

// matchRedirect returns the redirect target for requested: the registered
// URI it equals, or, for a registered loopback http URI, that URI on the
// port the request names. A native client binds an ephemeral port for its
// callback, so RFC 8252 §7.3 requires the server to allow any port on a
// loopback redirect. The target is built from the registered entry; only
// the port number comes from the request.
func matchRedirect(registered []string, requested string) (string, bool) {
	if i := slices.Index(registered, requested); i >= 0 {
		return registered[i], true
	}

	req, err := url.Parse(requested)
	if err != nil || req.Fragment != "" {
		return "", false
	}
	port, err := strconv.Atoi(req.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", false
	}

	for _, raw := range registered {
		reg, err := url.Parse(raw)
		if err != nil || !loopbackHTTP(reg) {
			continue
		}
		if reg.Scheme != req.Scheme || reg.Hostname() != req.Hostname() ||
			reg.Path != req.Path || reg.RawQuery != req.RawQuery {
			continue
		}
		reg.Host = net.JoinHostPort(reg.Hostname(), strconv.Itoa(port))

		return reg.String(), true
	}

	return "", false
}

// loopbackHTTP reports an http URL on localhost or a loopback address, the
// one shape validateRedirectURI admits for http.
func loopbackHTTP(u *url.URL) bool {
	if u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	a, err := netip.ParseAddr(host)

	return err == nil && a.IsLoopback()
}
