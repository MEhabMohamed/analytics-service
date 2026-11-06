// Package httpclient provides a reusable net/http wrapper with timeout, JSON unmarshaling, and retry-on-5xx.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/core-client/core-client.ts` + `pkg/utils/retry.ts`.
//   - In TypeScript, `fetch(url)` returns `Promise<Response>` and `await res.json() as T` casts at compile time.
//   - In Go, `DoJSON` takes `out any` (a pointer to the destination struct, e.g. `&env`) and populates it
//     via `json.Unmarshal(respBody, out)`.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a generic net/http wrapper with timeout, JSON decoding, and retry-on-5xx.
// It contains zero application-specific knowledge.
type Client struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	baseDelay  time.Duration
}

// Options configures a Client instance.
type Options struct {
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
	BaseDelay  time.Duration
}

// Request describes an outbound HTTP request.
type Request struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    any
}

// UpstreamError represents a non-2xx HTTP response from an upstream server.
type UpstreamError struct {
	StatusCode int
	Body       string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("upstream http %d: %s", e.StatusCode, e.Body)
}

// New constructs a Client with sensible defaults.
func New(opts Options) *Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	baseDelay := opts.BaseDelay
	if baseDelay <= 0 {
		baseDelay = 50 * time.Millisecond
	}
	return &Client{
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
	}
}

// DoJSON executes the HTTP request with retry-on-5xx/network-error and decodes the JSON response into out.
func (c *Client) DoJSON(ctx context.Context, req Request, out any) error {
	var bodyBytes []byte
	if req.Body != nil {
		b, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyBytes = b
	}

	url := c.baseURL + "/" + strings.TrimLeft(req.Path, "/")
	var lastErr error

	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bodyReader)
		if err != nil {
			return fmt.Errorf("build http request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		for k, v := range req.Headers {
			if v != "" {
				httpReq.Header.Set(k, v)
			}
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			if attempt < c.maxRetries && sleepCtx(ctx, c.backoff(attempt)) == nil {
				continue
			}
			return fmt.Errorf("http request failed: %w", lastErr)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < c.maxRetries && sleepCtx(ctx, c.backoff(attempt)) == nil {
				continue
			}
			return fmt.Errorf("read response body: %w", lastErr)
		}

		if resp.StatusCode >= 500 {
			lastErr = &UpstreamError{StatusCode: resp.StatusCode, Body: string(respBody)}
			if attempt < c.maxRetries && sleepCtx(ctx, c.backoff(attempt)) == nil {
				continue
			}
			return lastErr
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &UpstreamError{StatusCode: resp.StatusCode, Body: string(respBody)}
		}

		if resp.StatusCode == http.StatusNoContent || out == nil || len(respBody) == 0 {
			return nil
		}

		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response json: %w", err)
		}
		return nil
	}

	return lastErr
}

func (c *Client) backoff(attempt int) time.Duration {
	d := c.baseDelay * time.Duration(1<<(attempt-1))
	if d > 500*time.Millisecond {
		return 500 * time.Millisecond
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

