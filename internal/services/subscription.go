package services

import "context"

// TODO: Placeholder for subscribing to BBN events via websocket
func (s *Service) SubscribeToBbnEvents(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			}
		}
	}()
}
