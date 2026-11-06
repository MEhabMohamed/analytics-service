// Package coreclient implements the synchronous HTTP client to core-service.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/core-client/core-client.ts` (`CoreClient`).
//   - Automatically forwards `api-key` and `X-CorrelationId` (extracted from `ctx`) on every request.
package coreclient

import (
	"context"
	"time"

	"quickbite/analytics-service/lib/appcontext"
	"quickbite/analytics-service/pkg/httpclient"
)

// Client is the sync HTTP client to core-service (mirrors order-service/src/lib/core-client/core-client.ts).
type Client struct {
	http   *httpclient.Client
	apiKey string
}

// New constructs a core-service Client.
func New(baseURL string, apiKey string) *Client {
	return &Client{
		http: httpclient.New(httpclient.Options{
			BaseURL:    baseURL,
			Timeout:    5 * time.Second,
			MaxRetries: 3,
			BaseDelay:  50 * time.Millisecond,
		}),
		apiKey: apiKey,
	}
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	headers := map[string]string{
		"api-key": c.apiKey,
	}
	if corrID := appcontext.CorrelationIDFromContext(ctx); corrID != "" {
		headers["X-CorrelationId"] = corrID
	}
	return c.http.DoJSON(ctx, httpclient.Request{
		Method:  "GET",
		Path:    path,
		Headers: headers,
	}, out)
}

