package proxy

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var hopByHopHeaders = []string{
	"Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "TE", "Trailers",
	"Transfer-Encoding", "Upgrade",
}

// forwardRequest clones the incoming request and sends it to the upstream API.
func (s *Server) forwardRequest(r *http.Request) (*http.Response, error) {
	upstream := r.Clone(r.Context())
	parsed, err := url.Parse(s.upstream)
	if err != nil {
		return nil, fmt.Errorf("parsing upstream URL: %w", err)
	}
	upstream.URL.Scheme = parsed.Scheme
	upstream.URL.Host = parsed.Host
	upstream.Host = parsed.Host
	upstream.RequestURI = "" // must clear for http.Client

	for _, h := range hopByHopHeaders {
		upstream.Header.Del(h)
	}

	// Request gzip from upstream to save bandwidth
	upstream.Header.Set("Accept-Encoding", "gzip")

	client := &http.Client{
		Timeout: 300 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(upstream)
	if err != nil {
		return nil, fmt.Errorf("forwarding to upstream: %w", err)
	}

	// Transparently decompress gzip so SSE scanner can parse lines
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return resp, nil // fall through with raw body
		}
		resp.Body = &gzipReadCloser{gz: gzReader, underlying: resp.Body}
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
	}

	return resp, nil
}

// gzipReadCloser wraps a gzip reader and closes both layers.
type gzipReadCloser struct {
	gz         *gzip.Reader
	underlying io.ReadCloser
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.gz.Read(p)
}

func (g *gzipReadCloser) Close() error {
	g.gz.Close()
	return g.underlying.Close()
}

// copyResponseHeaders copies non-hop-by-hop headers from upstream response to client.
func copyResponseHeaders(w http.ResponseWriter, resp *http.Response) {
	hopSet := make(map[string]bool, len(hopByHopHeaders))
	for _, h := range hopByHopHeaders {
		hopSet[http.CanonicalHeaderKey(h)] = true
	}
	for key, values := range resp.Header {
		if hopSet[http.CanonicalHeaderKey(key)] {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
}
