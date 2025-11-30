package service

import (
	"net/http"
	"testing"
	"time"
)

func TestAIChatClientUsesExtendedTimeout(t *testing.T) {
	t.Parallel()

	client := newAIChatClient(nil, "gpt-4o", "deepseek-chat")

	httpClient, ok := client.http.(*http.Client)
	if !ok {
		t.Fatalf("expected *http.Client, got %T", client.http)
	}

	expectTimeout := 5 * time.Minute
	if httpClient.Timeout < expectTimeout {
		t.Fatalf("default timeout should be at least %v, got %v", expectTimeout, httpClient.Timeout)
	}

	client.SetHTTPClient(nil)
	httpClient, ok = client.http.(*http.Client)
	if !ok {
		t.Fatalf("expected *http.Client after reset, got %T", client.http)
	}
	if httpClient.Timeout < expectTimeout {
		t.Fatalf("reset timeout should be at least %v, got %v", expectTimeout, httpClient.Timeout)
	}
}
