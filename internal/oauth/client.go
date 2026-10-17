package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Bounds on a Client ID Metadata Document fetch (DESIGN-0023 §5): an
// authorization server fetching URLs a stranger chose needs every one.
const (
	clientFetchTimeout = 5 * time.Second
	clientMaxDocument  = 64 << 10
	clientMaxCache     = time.Hour
	clientCachePrefix  = "cimd:"
)

// ErrInvalidClient is wrapped by every refusal of a client id or its
// metadata document; the authorization endpoint reports it as
// invalid_client.
var ErrInvalidClient = errors.New("invalid client")

// ErrPrivateAddress is the SSRF guard's refusal: the client id's host
// resolved to a loopback, link-local, private, or unique-local address.
var ErrPrivateAddress = errors.New("host resolves to a private address")

// ClientMetadata is the Client ID Metadata Document, the fields docz-api
// reads. client_id must equal the URL the document was fetched from.
type ClientMetadata struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name,omitempty"`
	ClientURI    string   `json:"client_uri,omitempty"`
	RedirectURIs []string `json:"redirect_uris"`
}

// Name is what the consent page shows: the client's name, else its id.
func (c *ClientMetadata) Name() string {
	if c.ClientName != "" {
		return c.ClientName
	}

	return c.ClientID
}

// Resolver resolves a host name; *net.Resolver satisfies it and a test
// injects one that answers what it likes.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// ClientFetcherOptions configures a ClientFetcher.
type ClientFetcherOptions struct {
	// Cache holds fetched documents under cimd:<sha256(url)>; nil means a
	// MemoryCache.
	Cache Cache
	// Resolver resolves each host before the guard checks the addresses;
	// nil means net.DefaultResolver.
	Resolver Resolver
	// Transport, when set, replaces the guarded one entirely; for a test
	// against an httptest server.
	Transport http.RoundTripper
}

// ClientFetcher resolves a client id to its metadata document.
type ClientFetcher struct {
	cache Cache
	http  *http.Client
	now   func() time.Time
}

// NewClientFetcher builds a fetcher: https only, no redirects, a 5 s
// timeout, a 64 KiB cap, and a dialer that refuses private addresses.
func NewClientFetcher(opts *ClientFetcherOptions) *ClientFetcher {
	cache := opts.Cache
	if cache == nil {
		cache = NewMemoryCache()
	}
	resolver := opts.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	transport := opts.Transport
	if transport == nil {
		transport = &http.Transport{
			DialContext:         guardedDialer(resolver),
			TLSHandshakeTimeout: clientFetchTimeout,
			// A redirect is refused in CheckRedirect; a proxy would route around
			// the guard, so none is consulted.
			Proxy: nil,
		}
	}

	return &ClientFetcher{
		cache: cache,
		http: &http.Client{
			Transport: transport,
			Timeout:   clientFetchTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("redirects are not followed")
			},
		},
		now: time.Now,
	}
}

// guardedDialer resolves the host through resolver, refuses any private
// address, and dials the first public one.
func guardedDialer(resolver Resolver) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		addrs, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", host, err)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("resolve %s: no addresses", host)
		}

		for _, a := range addrs {
			if isPrivate(a) {
				return nil, fmt.Errorf("%s: %w", host, ErrPrivateAddress)
			}
		}

		var d net.Dialer

		return d.DialContext(ctx, network, net.JoinHostPort(addrs[0].Unmap().String(), port))
	}
}

// isPrivate is the address class the guard refuses: loopback, link-local
// (unicast and multicast), RFC 1918 and ULA, unspecified, and multicast.
func isPrivate(a netip.Addr) bool {
	a = a.Unmap()

	return a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsPrivate() || a.IsUnspecified() || a.IsMulticast() || a.IsInterfaceLocalMulticast()
}

// Resolve fetches, checks, and caches the document for clientID.
func (f *ClientFetcher) Resolve(ctx context.Context, clientID string) (ClientMetadata, error) {
	if err := validateClientID(clientID); err != nil {
		return ClientMetadata{}, fmt.Errorf("%w: %w", ErrInvalidClient, err)
	}

	key := clientCachePrefix + hashHex(clientID)
	if raw, err := f.cache.Get(ctx, key); err == nil {
		var meta ClientMetadata
		if json.Unmarshal(raw, &meta) == nil && meta.ClientID == clientID {
			return meta, nil
		}
	}

	raw, maxAge, err := f.fetch(ctx, clientID)
	if err != nil {
		return ClientMetadata{}, fmt.Errorf("%w: %w", ErrInvalidClient, err)
	}

	meta, err := parseClientMetadata(raw, clientID)
	if err != nil {
		return ClientMetadata{}, fmt.Errorf("%w: %w", ErrInvalidClient, err)
	}

	if maxAge > 0 {
		if stored, err := json.Marshal(meta); err == nil {
			if err := f.cache.Set(ctx, key, stored, min(maxAge, clientMaxCache)); err != nil {
				slog.DebugContext(ctx, "client metadata cache write failed", "client_id", clientID, "err", err)
			}
		}
	}

	return meta, nil
}

// validateClientID requires an https URL with a host and no fragment or
// userinfo; the document is fetched from exactly this string.
func validateClientID(clientID string) error {
	u, err := url.Parse(clientID)
	if err != nil {
		return errors.New("client_id is not a URL")
	}
	if u.Scheme != "https" {
		return errors.New("client_id must be an https URL")
	}
	if u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("client_id must be https://<host>/<path> with no userinfo or fragment")
	}

	return nil
}

// fetch GETs the document within the bounds and returns its bytes and the
// Cache-Control max-age (zero when absent).
func (f *ClientFetcher) fetch(ctx context.Context, clientID string) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, http.NoBody) //nolint:gosec // validated https URL; see guardedDialer
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.http.Do(req) //nolint:gosec // the transport dials through the private-address guard
	if err != nil {
		return nil, 0, fmt.Errorf("fetch client metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("fetch client metadata: status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, clientMaxDocument+1))
	if err != nil {
		return nil, 0, fmt.Errorf("read client metadata: %w", err)
	}
	if len(raw) > clientMaxDocument {
		return nil, 0, fmt.Errorf("client metadata exceeds %d bytes", clientMaxDocument)
	}

	return raw, maxAge(resp.Header.Get("Cache-Control")), nil
}

// parseClientMetadata decodes raw and checks what the flow depends on.
func parseClientMetadata(raw []byte, clientID string) (ClientMetadata, error) {
	var meta ClientMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return ClientMetadata{}, fmt.Errorf("client metadata is not JSON: %w", err)
	}
	if meta.ClientID != clientID {
		return ClientMetadata{}, errors.New("client metadata client_id does not match its URL")
	}
	if len(meta.RedirectURIs) == 0 {
		return ClientMetadata{}, errors.New("client metadata has no redirect_uris")
	}
	for _, u := range meta.RedirectURIs {
		if err := validateRedirectURI(u); err != nil {
			return ClientMetadata{}, err
		}
	}

	return meta, nil
}

// validateRedirectURI accepts an https URL, or an http URL on a loopback
// host, which is what a native MCP client listens on (OAuth 2.1 §8.4.2).
func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Fragment != "" {
		return fmt.Errorf("redirect_uri %q is not an absolute URL without a fragment", raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return nil
		}
		if a, err := netip.ParseAddr(host); err == nil && a.IsLoopback() {
			return nil
		}

		return fmt.Errorf("redirect_uri %q: http is allowed on loopback only", raw)
	default:
		return fmt.Errorf("redirect_uri %q: scheme must be https or loopback http", raw)
	}
}

// maxAge reads a Cache-Control header's max-age directive.
func maxAge(header string) time.Duration {
	for directive := range strings.SplitSeq(header, ",") {
		name, value, _ := strings.Cut(strings.TrimSpace(directive), "=")
		if !strings.EqualFold(name, "max-age") {
			continue
		}
		secs, err := strconv.Atoi(strings.Trim(value, `"`))
		if err != nil || secs <= 0 {
			return 0
		}

		return time.Duration(secs) * time.Second
	}

	return 0
}

// hashHex is the hex SHA-256 of s, the cache-key form of a URL.
func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))

	return hex.EncodeToString(sum[:])
}
