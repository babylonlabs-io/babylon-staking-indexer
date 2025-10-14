## Project Overview

This is backend service that indexes events from cosmos network called Babylon (bbn for short).

## Overview

- This project is written in Go
- Documentation located in `docs/` directory (for example state transition is described there)
- Configuration files are located in `config/` directory (`config-local.yml` is used for `make run-local`)

## Dependencies

- MongoDB as database
- babylon node
- BTC (Bitcoin) node
- RabbitMQ as message bus

## Development workflow

- Every time you've done with code changes run `make lint` to verify that linter doesn't complain about errors
- If there are formatting errors you must fix them by calling `make format`
- Do not modify mocks manually - always call `make generate` (it will generate code for interfaces using mockery). Mocks located in `tests/mocks/`
- If you want to run it locally just call `make run-local` (do not use make commands for docker)

## Testing

If you need to test that everything is working you should call one of these commands:
- `make test` for unit-tests
- `make test-integration` for tests related to database (note that it will setup docker container with mongo so it's a bit time consuming + it also will run unit-tests for now)
- `make test-e2e` major end-to-end test that setup all dependencies and verify that indexer works as expected (we also have CI build for every PR that runs this command)