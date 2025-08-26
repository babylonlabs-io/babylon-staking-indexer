package keybaseclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/client"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/rs/zerolog/log"
)

const endpoint = "/_/api/1.0/user/lookup.json"

const defaultMaxRetryTimes = 3
const defaultRetryInterval = 30 * time.Second
const defaultTimeout = 15 * time.Second

type Client struct {
	httpClient *http.Client
	cfg        *config.KeybaseConfig
}

func (c *Client) GetBaseURL() string {
	return "https://keybase.io"
}

func (c *Client) GetDefaultRequestTimeout() time.Duration {
	return defaultTimeout
}

func (c *Client) GetHttpClient() *http.Client {
	return c.httpClient
}

func NewClient(cfg *config.KeybaseConfig) *Client {
	if cfg == nil {
		return nil
	}

	return &Client{
		httpClient: &http.Client{},
		cfg:        cfg,
	}
}

func (c *Client) GetLogoURL(ctx context.Context, identity string) (string, error) {
	if identity == "" {
		return "", fmt.Errorf("empty identity provided")
	}

	type empty struct{}
	type lookupResponse struct {
		Status struct {
			Code int    `json:"code"`
			Name string `json:"name"`
		} `json:"status"`
		Them []struct {
			ID       string `json:"id"`
			Pictures struct {
				Primary struct {
					URL string `json:"url"`
				} `json:"primary"`
			} `json:"pictures"`
		} `json:"them"`
	}

	callForLogoURL := func() (string, error) {
		path := endpoint + fmt.Sprintf("?key_suffix=%s&fields=pictures&username=ds", identity)

		opts := &client.HttpClientOptions{
			Path:         path,
			TemplatePath: endpoint,
		}

		resp, err := client.SendRequest[empty, lookupResponse](ctx, c, http.MethodGet, opts, nil)
		if err != nil {
			fmt.Printf("SendRequest returned error: %v\n", err)
			return "", err
		}
		fmt.Printf("SendRequest succeeded, response: %+v\n", resp)

		if len(resp.Them) == 0 {
			return "", fmt.Errorf("no pictures found for %q", identity)
		}

		url := resp.Them[0].Pictures.Primary.URL
		if url == "" {
			return "", fmt.Errorf("empty picture url for %q (keybase changed response?)", identity)
		}

		return url, nil
	}

	result, err := clientCallWithRetry(ctx, callForLogoURL, c.cfg)
	if err != nil {
		return "", fmt.Errorf("failed to get logo URL for %q: %w", identity, err)
	}

	return result, nil
}

func clientCallWithRetry[T any](
	ctx context.Context,
	call retry.RetryableFuncWithData[T],
	cfg *config.KeybaseConfig,
) (T, error) {
	result, err := retry.DoWithData(call,
		retry.Context(ctx),
		retry.Attempts(defaultMaxRetryTimes),
		retry.Delay(defaultRetryInterval),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			// Only retry on rate limit errors (429)
			// Check if the error message contains "rate limit exceeded"
			shouldRetry := err != nil && strings.Contains(err.Error(), "rate limit exceeded")
			log.Ctx(ctx).Warn().
				Err(err).
				Bool("should_retry", shouldRetry).
				Msg("Retry condition check")
			return shouldRetry
		}),
		retry.OnRetry(func(n uint, err error) {
			log.Ctx(ctx).Debug().
				Uint("attempt", n+1).
				Uint("max_attempts", defaultMaxRetryTimes).
				Err(err).
				Msg("rate limit exceeded, retrying with exponential backoff")
		}))
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}
