package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestApplicationRequestContextDoesNotRetainPooledGinContext(t *testing.T) {
	type traceKey struct{}
	deadline := time.Now().Add(time.Minute)
	requestContext, cancel := context.WithDeadline(context.WithValue(context.Background(), traceKey{}, "trace-fixture"), deadline)
	defer cancel()
	pooled, _ := gin.CreateTestContext(httptest.NewRecorder())
	pooled.Request = httptest.NewRequest("POST", "/", nil).WithContext(requestContext)
	pooled.Set("user_id", "identity-is-resolved-before-domain-call")
	applicationContext := applicationRequestContext(pooled)
	if applicationContext != requestContext {
		t.Fatal("application retained the pooled framework context")
	}
	// Simulate reuse after response; the database must still see the original
	// request's cancellation, deadline and trace context, never the next request.
	pooled.Request = httptest.NewRequest("GET", "/other", nil)
	pooled.Set("user_id", "other-user")
	cancel()
	if !errors.Is(applicationContext.Err(), context.Canceled) {
		t.Fatal("request cancellation was lost")
	}
	if actual, ok := applicationContext.Deadline(); !ok || !actual.Equal(deadline) {
		t.Fatal("request deadline was lost")
	}
	if applicationContext.Value(traceKey{}) != "trace-fixture" {
		t.Fatal("request trace context was lost")
	}
}

func TestApplicationRequestContextPreservesNonHTTPTransactionContext(t *testing.T) {
	type transactionKey struct{}
	ctx := context.WithValue(context.Background(), transactionKey{}, "transaction")
	if applicationRequestContext(ctx) != ctx {
		t.Fatal("non-HTTP context was replaced")
	}
}
