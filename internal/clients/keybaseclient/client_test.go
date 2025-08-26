package keybaseclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testClient is a wrapper around Client that allows overriding the base URL for testing
type testClient struct {
	*Client
	baseURL string
}

func (tc *testClient) GetBaseURL() string {
	return tc.baseURL
}

func TestGetLogoURL_WithRetry(t *testing.T) {
	// Initialize metrics for testing
	metrics.Init(9999)

	// Test configuration
	cfg := &config.KeybaseConfig{
		MaxRetryTimes: 3,
		RetryInterval: 10 * time.Millisecond, // Short interval for testing
		Timeout:       5 * time.Second,
	}

	// Create a test server that returns 429 for the first 2 requests, then 200
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		fmt.Printf("Request %d received at %s\n", requestCount, r.URL.String())
		t.Logf("Request %d received", requestCount)
		fmt.Printf("About to check if requestCount (%d) <= 2\n", requestCount)
		if requestCount <= 2 {
			// Return 429 for first 2 requests
			t.Logf("Returning 429 for request %d", requestCount)
			fmt.Printf("Returning 429 for request %d\n", requestCount)
			w.WriteHeader(http.StatusTooManyRequests)
			responseBody := `{"status":{"code":429,"name":"Too Many Requests"}}`
			fmt.Printf("429 response body: %s\n", responseBody)
			w.Write([]byte(responseBody))
			return // Make sure we return here to avoid writing the success response
		} else {
			// Return success response
			t.Logf("Returning 200 for request %d", requestCount)
			fmt.Printf("Returning 200 for request %d\n", requestCount)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": {"code": 0, "name": "OK"},
				"them": [{
					"id": "test_id",
					"pictures": {
						"primary": {
							"url": "https://example.com/logo.png"
						}
					}
				}]
			}`))
		}
	}))
	defer server.Close()

	// Create a test client that overrides the base URL
	testClient := &testClient{
		Client: &Client{
			httpClient: &http.Client{},
			cfg:        cfg,
		},
		baseURL: server.URL,
	}

	// Test the retry logic
	ctx := context.Background()

	// First, let's verify the testClient is working
	fmt.Printf("Test client base URL: %s\n", testClient.GetBaseURL())
	fmt.Printf("Server URL: %s\n", server.URL)

	// Let's test a direct HTTP request to the test server
	resp, err := http.Get(server.URL + "/_/api/1.0/user/lookup.json?key_suffix=test_identity&fields=pictures&username=ds")
	if err != nil {
		t.Fatalf("Direct HTTP request failed: %v", err)
	}
	fmt.Printf("Direct HTTP response status: %d\n", resp.StatusCode)
	resp.Body.Close()

	logoURL, err := testClient.GetLogoURL(ctx, "test_identity")

	// Should succeed after retries
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/logo.png", logoURL)
	assert.Equal(t, 3, requestCount, "Should have made 3 requests (2 failures + 1 success)")
}

func TestGetLogoURL_ExceedsMaxRetries(t *testing.T) {
	// Initialize metrics for testing
	metrics.Init(9999)

	// Test configuration with fewer retries
	cfg := &config.KeybaseConfig{
		MaxRetryTimes: 2,
		RetryInterval: 10 * time.Millisecond,
		Timeout:       5 * time.Second,
	}

	// Create a test server that always returns 429
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"status":{"code":429,"name":"Too Many Requests"}}`))
	}))
	defer server.Close()

	// Create a test client that overrides the base URL
	testClient := &testClient{
		Client: &Client{
			httpClient: &http.Client{},
			cfg:        cfg,
		},
		baseURL: server.URL,
	}

	// Test that it fails after max retries
	ctx := context.Background()
	_, err := testClient.GetLogoURL(ctx, "test_identity")

	// Should fail after max retries
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get logo URL")
	assert.Equal(t, 2, requestCount, "Should have made 2 requests before giving up")
}

func TestGetLogoURL_EmptyIdentity(t *testing.T) {
	// Initialize metrics for testing
	metrics.Init(9999)

	cfg := &config.KeybaseConfig{
		MaxRetryTimes: 3,
		RetryInterval: 1 * time.Second,
		Timeout:       15 * time.Second,
	}

	client := NewClient(cfg)
	ctx := context.Background()

	_, err := client.GetLogoURL(ctx, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty identity provided")
}

func TestGetLogoURL_NoPicturesFound(t *testing.T) {
	// Initialize metrics for testing
	metrics.Init(9999)

	cfg := &config.KeybaseConfig{
		MaxRetryTimes: 3,
		RetryInterval: 1 * time.Second,
		Timeout:       15 * time.Second,
	}

	// Create a test server that returns empty results
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": {"code": 0, "name": "OK"},
			"them": []
		}`))
	}))
	defer server.Close()

	testClient := &testClient{
		Client: &Client{
			httpClient: &http.Client{},
			cfg:        cfg,
		},
		baseURL: server.URL,
	}

	ctx := context.Background()
	_, err := testClient.GetLogoURL(ctx, "test_identity")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pictures found")
}

func TestGetLogoURL_ExponentialBackoff(t *testing.T) {
	// Initialize metrics for testing
	metrics.Init(9999)

	// Test configuration with longer retry interval to observe backoff
	cfg := &config.KeybaseConfig{
		MaxRetryTimes: 3,
		RetryInterval: 50 * time.Millisecond, // Base interval
		Timeout:       5 * time.Second,
	}

	// Track request timestamps to verify exponential backoff
	var requestTimes []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())

		if len(requestTimes) <= 2 {
			// Return 429 for first 2 requests
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"status":{"code":429,"name":"Too Many Requests"}}`))
		} else {
			// Return success response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": {"code": 0, "name": "OK"},
				"them": [{
					"id": "test_id",
					"pictures": {
						"primary": {
							"url": "https://example.com/logo.png"
						}
					}
				}]
			}`))
		}
	}))
	defer server.Close()

	// Create a test client that overrides the base URL
	testClient := &testClient{
		Client: &Client{
			httpClient: &http.Client{},
			cfg:        cfg,
		},
		baseURL: server.URL,
	}

	// Test the exponential backoff logic
	ctx := context.Background()
	logoURL, err := testClient.GetLogoURL(ctx, "test_identity")

	// Should succeed after retries
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/logo.png", logoURL)
	assert.Equal(t, 3, len(requestTimes), "Should have made 3 requests")

	// Verify exponential backoff: delays should increase
	if len(requestTimes) >= 3 {
		delay1 := requestTimes[1].Sub(requestTimes[0])
		delay2 := requestTimes[2].Sub(requestTimes[1])

		// With exponential backoff, delay2 should be longer than delay1
		// Allow some tolerance for timing variations
		assert.Greater(t, delay2, delay1, "Second delay should be longer than first delay due to exponential backoff")
	}
}

func TestRetryLogic_Simple(t *testing.T) {
	// Test the retry logic with a simple function that returns a rate limit error
	cfg := &config.KeybaseConfig{
		MaxRetryTimes: 3,
		RetryInterval: 10 * time.Millisecond,
		Timeout:       5 * time.Second,
	}

	callCount := 0
	testFunc := func() (string, error) {
		callCount++
		if callCount <= 2 {
			return "", fmt.Errorf("rate limit exceeded when calling test")
		}
		return "success", nil
	}

	ctx := context.Background()
	result, err := clientCallWithRetry(ctx, testFunc, cfg)

	require.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 3, callCount, "Should have called the function 3 times")
}

func TestBaseClient_429Handling(t *testing.T) {
	// Initialize metrics for testing
	metrics.Init(9999)

	// Test that the base client properly handles 429 status codes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Test server received request: %s\n", r.URL.String())
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"status":{"code":429,"name":"Too Many Requests"}}`))
		fmt.Printf("Test server sent 429 response\n")
	}))
	defer server.Close()

	client := &Client{
		httpClient: &http.Client{},
		cfg: &config.KeybaseConfig{
			MaxRetryTimes: 1,
			RetryInterval: 10 * time.Millisecond,
			Timeout:       5 * time.Second,
		},
	}

	testClient := &testClient{
		Client:  client,
		baseURL: server.URL,
	}

	ctx := context.Background()

	// First, let's test a direct HTTP request to the test server
	resp, err := http.Get(server.URL + "/_/api/1.0/user/lookup.json?key_suffix=test_identity&fields=pictures&username=ds")
	if err != nil {
		t.Fatalf("Direct HTTP request failed: %v", err)
	}
	fmt.Printf("Direct HTTP response status: %d\n", resp.StatusCode)
	resp.Body.Close()

	_, err = testClient.GetLogoURL(ctx, "test_identity")

	// Should fail with rate limit error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")
}

func TestNewClient_WithNilConfig(t *testing.T) {
	// Test that the client works correctly with a nil config
	client := NewClient(nil)

	// Should use default config
	assert.NotNil(t, client.cfg)
	assert.Equal(t, uint(3), client.cfg.MaxRetryTimes)
	assert.Equal(t, 1*time.Second, client.cfg.RetryInterval)
	assert.Equal(t, 15*time.Second, client.cfg.Timeout)
}
