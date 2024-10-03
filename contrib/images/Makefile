DOCKER = $(shell which docker)

.PHONY: babylon-staking-indexer babylon-staking-indexer-rmi

babylon-staking-indexer:
	$(DOCKER) build --tag babylonlabs-io/babylon-staking-indexer -f babylon-staking-indexer/Dockerfile \
				$(shell git rev-parse --show-toplevel)

babylon-staking-indexer-rmi:
	$(DOCKER) rmi babylonlabs-io/babylon-staking-indexer 2>/dev/null; true
