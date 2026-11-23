package service

import (
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"
)

// Bounded retry for the Go -> ai-service client (R2-13/#636).
//
// Only calls whose replay is safe opt in, per the classification pinned
// by the endpoint methods and ai_client_retry_test.go:
//
//   - (a) idempotent reads: knowledge search and knowledge stats;
//   - (b) writes carrying a dedup identity: runtime erasures (keyed by
//     the resource id in the path plus the content fingerprint) and
//     video ingestion (pinned by source key and expected content
//     hash).
//
// Unkeyed generation/upload calls (assessment, diagnosis, treatment,
// episodic-shadow proposals, OCR, posture) and the streaming
// consultation turns never retry: a replay could double-spend the free
// model quota or duplicate a side effect, and their callers own the
// retry decision at the job layer. Checkpoint cleanup is likewise
// excluded although its endpoint is idempotent: the chat-retention
// prune job owns a durable retry budget that pins exactly one HTTP
// call per job attempt, and the endpoint idempotency is what makes
// those job-level retries safe.

// aiRetryPolicy bounds the retry behaviour of one logical call.
type aiRetryPolicy struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	// budget bounds the retry window measured from the first attempt: a
	// further attempt only starts while elapsed time plus the next
	// backoff stays within budget. It never truncates an in-flight
	// attempt (the client timeout owns that), so fast calls get a hard
	// total cap far below the 180s model-gateway timeout, while a slow
	// first attempt (video ingestion) simply forfeits its retries.
	budget time.Duration
}

var defaultAIRetryPolicy = aiRetryPolicy{
	maxAttempts: 3,
	baseDelay:   200 * time.Millisecond,
	maxDelay:    2 * time.Second,
	budget:      10 * time.Second,
}

func (p aiRetryPolicy) delayAfter(failedAttempt int) time.Duration {
	backoff := p.baseDelay << (failedAttempt - 1)
	if backoff > p.maxDelay || backoff <= 0 {
		backoff = p.maxDelay
	}
	// Full jitter: sleep somewhere in [0, backoff].
	return time.Duration(rand.Int64N(int64(backoff) + 1))
}

// retryableAIStatus reports whether an HTTP status is a transient
// upstream failure worth another attempt. 429 is deliberately absent:
// the free model quota is precious and a rate-limit answer will not
// clear inside the retry window. The ai-service sends no Retry-After
// header today, so none is honoured here.
func retryableAIStatus(status int) bool {
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// doRetryable executes makeRequest until a final outcome: success, a
// non-retryable result, context cancellation, or the attempt/window
// budget running out. It returns the final response with its body still
// open (the caller owns it), or the final transport error. Construction
// errors from makeRequest are returned verbatim and never retried.
// Requests of non-retryable calls are executed exactly once.
func (c *AIClient) doRetryable(
	ctx context.Context,
	operation string,
	retryable bool,
	makeRequest func() (*http.Request, error),
) (*http.Response, error) {
	policy := c.retryPolicy
	if policy.maxAttempts <= 0 {
		policy = defaultAIRetryPolicy
	}
	started := time.Now()
	for attempt := 1; ; attempt++ {
		req, err := makeRequest()
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err == nil && !(retryable && retryableAIStatus(resp.StatusCode)) {
			return resp, nil
		}
		// The outcome is retryable in principle: a transport error or a
		// transient upstream status on a call that opted in.
		if !retryable || attempt >= policy.maxAttempts {
			if attempt > 1 {
				logAIRetryExhausted(operation, attempt, resp, err)
			}
			if err != nil {
				return nil, err
			}
			return resp, nil
		}
		delay := policy.delayAfter(attempt)
		if time.Since(started)+delay > policy.budget {
			if attempt > 1 {
				logAIRetryExhausted(operation, attempt, resp, err)
			}
			if err != nil {
				return nil, err
			}
			return resp, nil
		}
		if err != nil {
			slog.Default().Warn(
				"ai service request retrying",
				"operation", operation,
				"attempt", attempt,
				"error", err,
			)
		} else {
			slog.Default().Warn(
				"ai service request retrying",
				"operation", operation,
				"attempt", attempt,
				"status", resp.StatusCode,
			)
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func logAIRetryExhausted(operation string, attempts int, resp *http.Response, err error) {
	if err != nil {
		slog.Default().Error(
			"ai service request failed after retries",
			"operation", operation,
			"attempts", attempts,
			"error", err,
		)
		return
	}
	slog.Default().Error(
		"ai service request failed after retries",
		"operation", operation,
		"attempts", attempts,
		"status", resp.StatusCode,
	)
}

// cloneAIRequest rebuilds a request for another attempt, replaying the
// body from GetBody (set for every byte-backed body this client sends).
func cloneAIRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		clone.Body = body
	}
	return clone, nil
}
