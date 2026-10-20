package recipes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// fetcher downloads images on behalf of agents ("use this photo URL"). Since
// the URL comes from an agent and the server may sit on a private network,
// it must never be usable to reach internal services (SSRF), so:
//   - only http and https URLs,
//   - the connection itself is refused for loopback, private, link-local and
//     other non-public addresses. The check runs in the dialer's Control
//     hook, on the address actually being connected to, so DNS tricks and
//     redirects to internal hosts are covered too,
//   - no proxy, a few redirects at most, short timeouts and a size cap.
type fetcher struct {
	client *http.Client
}

func newFetcher(allowPrivate bool) *fetcher {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || (!allowPrivate && !isPublicIP(ip)) {
				return errors.New("address not allowed")
			}
			return nil
		},
	}
	return &fetcher{client: &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			MaxIdleConns:          2,
			DisableKeepAlives:     true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return errors.New("too many redirects")
			}
			return checkScheme(req.URL)
		},
	}}
}

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func isPublicIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	switch {
	case ip.IsLoopback(), ip.IsPrivate(), ip.IsUnspecified(),
		ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast(), ip.IsMulticast(),
		ip.IsInterfaceLocalMulticast(), cgnat.Contains(ip):
		return false
	}
	return true
}

func checkScheme(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are supported")
	}
	return nil
}

// fetch returns the response body, capped at MaxImageBytes. Failures come back
// as ErrInvalid so the message reaches the agent.
func (f *fetcher) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return nil, invalid("not a valid URL")
	}
	if err := checkScheme(u); err != nil {
		return nil, invalid("%v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, invalid("not a valid URL")
	}
	req.Header.Set("Accept", "image/jpeg,image/png,image/webp")
	req.Header.Set("User-Agent", "Cookira/1 (+recipe image fetch)")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, invalid("could not download the image: %v", unwrapURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, invalid("could not download the image: server returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return nil, invalid("could not download the image: %v", err)
	}
	if len(data) > MaxImageBytes {
		return nil, invalid("image is larger than %d MB", MaxImageBytes>>20)
	}
	return data, nil
}

func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}
