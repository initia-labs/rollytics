# rollytics

Rollytics is an analytics and indexing tool designed for the Minitia ecosystem. It provides comprehensive data collection, processing, and API services for blockchain networks, supporting multiple VM types (Move, Wasm, EVM).

## Features
- Minitia data indexing and analytics
- RESTful API server for data access
- Support for Move, Wasm, and EVM based minitias
- Flexible configuration via CLI flags or environment variables
- Database auto-migration and batch processing

## Requirements
- Go 1.24+
- Docker (optional, for containerized deployment)
- Supported database (e.g., PostgreSQL)

## Installation

### Build from Source
```sh
git clone https://github.com/initia-labs/rollytics.git
cd rollytics
make install
```
This will produce the `rollytics` binary in $GOBIN.

### Docker
```sh
docker build -t rollytics .
```

## Configuration
You can configure rollytics using CLI flags or environment variables. CLI flags take precedence.

### Common Options
- `--log_format` (`plain`|`json`): Log output format (default: `plain`)
- `--log_level` (`debug`|`info`|`warn`|`error`): Log level (default: `warn`)

### Example
```sh
export DB_DSN='postgres://user:pass@tcp(localhost:5432)/db'
export CHAIN_ID='myminitia-1'
export VM_TYPE='evm'
export RPC_URL='http://localhost:26657'
export REST_URL='http://localhost:1317'
export PORT='8080'
export LOG_FORMAT='json'
export LOG_LEVEL='info'
./rollytics api
```

## Usage

### API Server
Start the API server:
```sh
./rollytics api
```

### Indexer
Start the indexer:
```sh
./rollytics indexer
```

## Development
- Run tests: `make test`
- Lint: `make lint`
- Generate Swagger docs: `make swagger-gen`

## License
- Proprietary. All rights reserved. 

## Changelog
For full version history, see **[CHANGELOG.md](CHANGELOG.md)**.
