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
- PostgreSQL 15+ compatible

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
export JSON_RPC_URL='http://localhost:8545'
export RPC_URL='http://localhost:26657'
export REST_URL='http://localhost:1317'
export PORT='8080'
export LOG_FORMAT='json'
export LOG_LEVEL='info'
./rollytics api
```

## Usage

### API Server
Start the API server :
```sh
./rollytics api
```

By Docker :
```sh
# Run API server with Docker
docker run -d \
  --name rollytics-api \
  -p 8080:8080 \
  -e DB_DSN= $DB_DSN\
  -e DB_MAX_CONNS=10 \
  -e DB_IDLE_CONNS=10 \
  -e COOLING_DURATION=100ms \
  -e CHAIN_ID=$CHAIN_ID \
  -e VM_TYPE=$VM_TYPE \
  -e RPC_URL=$RPC_URL \
  -e REST_URL=$REST_URL \
  -e JSON_RPC_URL=$JSON_RPC_URL \
  -e LOG_LEVEL='info' \
  -e PORT='3000' \
  -e DB_AUTO_MIGRATE='false' \
  rollytics:$VERSION api

# Check logs
docker logs -f rollytics-api
```

### Indexer
Start the indexer :
```sh
./rollytics indexer
```
By Docker :
```sh
# Run indexer with Docker
docker run -d \
  --name rollytics-indexer \
  -p 3000:3000 \
  -e DB_DSN=$DB_DSN \
  -e DB_MAX_CONNS=10 \
  -e DB_IDLE_CONNS=10 \
  -e COOLING_DURATION=100ms \
  -e CHAIN_ID=$CHAIN_ID \
  -e VM_TYPE=$VM_TYPE \
  -e RPC_URL=$RPC_URL \
  -e REST_URL=$REST_URL \
  -e JSON_RPC_URL=$JSON_RPC_URL \
  -e LOG_LEVEL='info' \
  -e PORT='3000' \
  -e DB_AUTO_MIGRATE='true' \
  rollytics:$VERSION indexer

# Check logs
docker logs -f rollytics-indexer
```

## Development
- Run tests: `make test`
- Lint: `make lint`
- Generate Swagger docs: `make swagger-gen`

## License
- Proprietary. All rights reserved. 

## Changelog
For full version history, see **[CHANGELOG.md](CHANGELOG.md)**.
