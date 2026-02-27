package services

import (
	"context"
	"fmt"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/rs/zerolog/log"
)

const (
	cacheCleanupInterval = 5 * time.Minute
	cacheTTL             = 30 * time.Minute
	maxCacheSize         = 10000
)

type cacheEntry struct {
	delegation *model.BTCDelegationDetails
	cachedAt   time.Time
}

type DelegationCache struct {
	entries map[string]*cacheEntry
	hits    int64
	misses  int64
}

func NewDelegationCache() *DelegationCache {
	return &DelegationCache{
		entries: make(map[string]*cacheEntry),
	}
}

// Get retrieves a delegation from cache
func (c *DelegationCache) Get(stakingTxHash string) (*model.BTCDelegationDetails, bool) {
	entry, exists := c.entries[stakingTxHash]
	if !exists {
		c.misses++
		return nil, false
	}

	// Check if entry has expired
	if time.Since(entry.cachedAt) > cacheTTL {
		delete(c.entries, stakingTxHash)
		c.misses++
		return nil, false
	}

	c.hits++
	return entry.delegation, true
}

// Set adds a delegation to cache
func (c *DelegationCache) Set(stakingTxHash string, delegation *model.BTCDelegationDetails) {
	// Evict oldest entries if cache is full
	if len(c.entries) >= maxCacheSize {
		c.evictOldest()
	}

	c.entries[stakingTxHash] = &cacheEntry{
		delegation: delegation,
		cachedAt:   time.Now(),
	}
}

// Delete removes a delegation from cache
func (c *DelegationCache) Delete(stakingTxHash string) {
	delete(c.entries, stakingTxHash)
}

// StartCleanup starts a background goroutine to clean expired entries
func (c *DelegationCache) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(cacheCleanupInterval)
		for {
			select {
			case <-ticker.C:
				c.cleanup()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (c *DelegationCache) cleanup() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.Sub(entry.cachedAt) > cacheTTL {
			delete(c.entries, key)
		}
	}
}

func (c *DelegationCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.cachedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.cachedAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// GetHitRate returns the cache hit rate
func (c *DelegationCache) GetHitRate() float64 {
	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}

// GetOrFetch retrieves from cache or fetches from database
func (s *Service) GetOrFetchDelegation(
	ctx context.Context,
	stakingTxHash string,
) (*model.BTCDelegationDetails, error) {
	if s.delegationCache != nil {
		if delegation, ok := s.delegationCache.Get(stakingTxHash); ok {
			return delegation, nil
		}
	}

	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get delegation: %w", err)
	}

	if s.delegationCache != nil {
		s.delegationCache.Set(stakingTxHash, delegation)
	}

	return delegation, nil
}

// BatchGetDelegations retrieves multiple delegations, using cache where possible
func (s *Service) BatchGetDelegations(
	ctx context.Context,
	stakingTxHashes []string,
) ([]*model.BTCDelegationDetails, error) {
	results := make([]*model.BTCDelegationDetails, len(stakingTxHashes))

	for i := 0; i < len(stakingTxHashes); i++ {
		delegation, err := s.GetOrFetchDelegation(ctx, stakingTxHashes[i])
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).
				Str("staking_tx_hash", stakingTxHashes[i]).
				Msg("failed to fetch delegation, skipping")
		}
		results[i] = delegation
	}

	return results, nil
}

// PurgeDelegationCache invalidates all entries for given finality provider
func (s *Service) PurgeDelegationCache(fpBtcPkHex string) int {
	if s.delegationCache == nil {
		return 0
	}

	purged := 0
	for key, entry := range s.delegationCache.entries {
		for _, fpPk := range entry.delegation.FinalityProviderBtcPksHex {
			if fpPk == fpBtcPkHex {
				delete(s.delegationCache.entries, key)
				purged++
			}
		}
	}

	return purged
}
