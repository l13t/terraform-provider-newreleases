package client

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// retryTransport retries requests rejected with HTTP 429 (rate limit) or
// HTTP 503 (maintenance). The newreleases client has no retry logic of its
// own, and write requests cost 10 rate-limit units each, so an apply touching
// dozens of projects would otherwise fail halfway through.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
	maxBackoff time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	for attempt := 0; ; attempt++ {
		resp, err := t.base.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable {
			return resp, nil
		}
		if attempt >= t.maxRetries {
			return resp, nil
		}
		// A consumed body that cannot be recreated must not be replayed.
		if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
			return resp, nil
		}

		delay := t.delay(resp, attempt)

		// Drain so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		tflog.Warn(ctx, "newreleases: retrying after rate limit", map[string]any{
			"status":  resp.StatusCode,
			"delay":   delay.String(),
			"attempt": attempt + 1,
		})

		if err := sleep(ctx, delay); err != nil {
			return nil, err
		}

		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}
	}
}

// delay honours an integer Retry-After header and otherwise falls back to
// exponential backoff, capped at maxBackoff.
func (t *retryTransport) delay(resp *http.Response, attempt int) time.Duration {
	d := time.Second << attempt
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
		d = time.Duration(s) * time.Second
	}
	return min(d, t.maxBackoff)
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
