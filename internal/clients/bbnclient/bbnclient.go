package bbnclient

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	baseclient "github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/base"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient/bbntypes"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
)

type BbnClient struct {
	config         *config.BbnConfig
	defaultHeaders map[string]string
	httpClient     *http.Client
}

func NewBbnClient(cfg *config.BbnConfig) *BbnClient {
	httpClient := &http.Client{}
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
	return &BbnClient{
		cfg,
		headers,
		httpClient,
	}
}

// Necessary for the BaseClient interface
func (c *BbnClient) GetBaseURL() string {
	return fmt.Sprintf("https://%s:%s", c.config.Endpoint, c.config.Port)
}

func (c *BbnClient) GetDefaultRequestTimeout() int {
	return c.config.Timeout
}

func (c *BbnClient) GetHttpClient() *http.Client {
	return c.httpClient
}

func (c *BbnClient) GetHealthCheckStatus(ctx context.Context) (bool, *types.Error) {
	path := "/health"
	opts := &baseclient.BaseClientOptions{
		Path:         path,
		TemplatePath: path,
		Headers:      c.defaultHeaders,
	}

	_, err := baseclient.SendRequest[any, any](
		ctx, c, http.MethodGet, opts, nil,
	)
	return err == nil, err
}

func (c *BbnClient) GetLatestBlockNumber(ctx context.Context) (int, *types.Error) {
	blockResult, err := c.getBlockResults(ctx, 0)
	if err != nil {
		return 0, err
	}
	// Parse the string as an unsigned integer (base 10)
	height, parseErr := strconv.Atoi(blockResult.Height)
	if parseErr != nil {
		return 0, types.NewErrorWithMsg(
			http.StatusInternalServerError, types.InternalServiceError, parseErr.Error(),
		)
	}

	return height, nil
}

func (c *BbnClient) GetBlockResults(ctx context.Context, height int) (*bbntypes.BlockResultsResponse, *types.Error) {
	return c.getBlockResults(context.Background(), height)
}

func (c *BbnClient) getBlockResults(ctx context.Context, blockHeight int) (*bbntypes.BlockResultsResponse, *types.Error) {
	path := "/block_results"
	if blockHeight > 0 {
		path = fmt.Sprintf("/block_results?height=%d", blockHeight)
	}
	opts := &baseclient.BaseClientOptions{
		Path:         path,
		TemplatePath: path,
		Headers:      c.defaultHeaders,
	}

	resp, err := baseclient.SendRequest[
		any, bbntypes.CometBFTRPCResponse[bbntypes.BlockResultsResponse],
	](
		ctx, c, http.MethodGet, opts, nil,
	)
	if err != nil {
		return nil, err
	}
	return &resp.Result, nil
}
