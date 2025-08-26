package services

import (
	"context"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils/poller"
)

// Logo renewal interval is 7 days
const logoRenewalInterval = 7 * 24 * time.Hour

func (s *Service) SyncLogos(ctx context.Context) {
	logoPoller := poller.NewPoller(
		s.cfg.Poller.LogoPollingInterval,
		metrics.RecordPollerDuration("fetch_and_save_logos", s.fetchAndSaveLogos),
	)
	go logoPoller.Start(ctx)
}

func (s *Service) fetchAndSaveLogos(ctx context.Context) error {
	// Step 1: Fetch all finality providers from the database
	fp, err := s.db.GetAllFinalityProviders(ctx)
	if err != nil {
		return err
	}

	// Step 2: For each finality provider, fetch the logo from the Keybase API
	for _, fp := range fp {
		// Some FPs don't have an identity, so we skip them
		if fp.Description.Identity == "" {
			continue
		}
		// If the logo is already up to date, we skip it
		if fp.Logo.URL != "" && fp.Logo.LastUpdatedAt.After(
			time.Now().Add(-logoRenewalInterval),
		) {
			continue
		}
		logo, err := s.keybase.GetLogoURL(ctx, fp.Description.Identity)
		if err != nil {
			return err
		}

		// Insert the logo into the database
		err = s.db.UpdateFinalityProviderLogo(ctx, fp.BtcPk, logo)
		if err != nil {
			return err
		}
	}

	// Step 3: Save the logo to the database
	return nil
}
