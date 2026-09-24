// Package client builds the newreleases.io API client used by the provider.
package client

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"newreleases.io/newreleases"
)

// Config holds the connection settings for the API client.
type Config struct {
	APIKey string
	// BaseURL overrides the API endpoint; empty uses the client default
	// (https://api.newreleases.io/).
	BaseURL string
}

// New returns a newreleases client whose transport retries rate-limited and
// maintenance responses.
func New(cfg Config) (*newreleases.Client, error) {
	var baseURL *url.URL
	if cfg.BaseURL != "" {
		u, err := url.Parse(cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("parsing base_url %q: %w", cfg.BaseURL, err)
		}
		baseURL = u
	}

	opts := &newreleases.ClientOptions{
		// NewClient wraps this Transport with its own authenticating
		// RoundTripper, so the retry layer sees final absolute URLs.
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &retryTransport{
				base:       http.DefaultTransport,
				maxRetries: 3,
				maxBackoff: 60 * time.Second,
			},
		},
		BaseURL: baseURL,
	}
	return newreleases.NewClient(cfg.APIKey, opts), nil
}
