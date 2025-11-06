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

#### Option 1: Pull from GitHub Container Registry

```sh
# Pull latest version
docker pull ghcr.io/initia-labs/rollytics:latest

# Or pull specific version
docker pull ghcr.io/initia-labs/rollytics:v1.0.0
```

#### Option 2: Build from Source

```sh
docker build -t rollytics .
```

## Configuration

You can configure rollytics using environment variables. All settings can be configured via environment variables or .env file.

### Core Settings

- **`CHAIN_ID`: Chain ID (required)**
- **`VM_TYPE`: Virtual machine type - `move`, `wasm`, or `evm` (required, affects default settings)**
- **`RPC_URL`: Tendermint RPC URL (required)**
- **`REST_URL`: Cosmos REST API URL (required)**
- **`JSON_RPC_URL`: JSON-RPC URL for EVM chains (required for EVM)**
- `ACCOUNT_ADDRESS_PREFIX`: Address prefix (optional, default: `init`)

### Server Settings

- `PORT`: API server port (optional, default: `8080`)
- `LOG_FORMAT`: Log format - `plain` or `json` (optional, default: `json`)
- `LOG_LEVEL`: Log level - `debug`, `info`, `warn`, `error` (optional, default: `warn`)

### Database Settings

- **`DB_DSN`: Database connection string (required)**
- `DB_MAX_CONNS`: Maximum database connections (optional, default: `0` - unlimited)
- `DB_IDLE_CONNS`: Idle database connections (optional, default: `2`)
- `DB_BATCH_SIZE`: Batch insert size (optional, default: `100`)
- `DB_AUTO_MIGRATE`: Auto-migrate database schema (optional, default: `false`)
- `DB_MIGRATION_DIR`: Migration files directory (optional, default: `orm/migrations`)

### Performance Settings

- `COOLING_DURATION`: Cooling period between operations (optional, default: `50ms`)
- `QUERY_TIMEOUT`: Query timeout duration (optional, default: `30s`)
- `MAX_CONCURRENT_REQUESTS`: Maximum concurrent requests (optional, default: `50`, max: `1000`)
- `POLLING_INTERVAL`: API polling interval (optional, default: `3s`)

### Cache Settings

- `CACHE_SIZE`: General cache size (optional, default: `1000`)
- `CACHE_TTL`: Cache time-to-live (optional, default: `10m`)
- `ACCOUNT_CACHE_SIZE`: Account cache size (optional, default: `40960`)
- `NFT_CACHE_SIZE`: NFT cache size (optional, default: `40960`)
- `MSG_TYPE_CACHE_SIZE`: Message type cache size (optional, default: `1024`)
- `TYPE_TAG_CACHE_SIZE`: Type tag cache size (optional, default: `1024`)
- `EVM_TX_HASH_CACHE_SIZE`: EVM transaction hash cache size (optional, default: `40960`)

### Internal Transaction Settings

- `INTERNAL_TX`: Enable internal transaction tracking (optional, default: `true` for EVM, `false` for Move/Wasm)
- `INTERNAL_TX_POLL_INTERVAL`: Internal TX polling interval (optional, default: `5s`)
- `INTERNAL_TX_BATCH_SIZE`: Internal TX batch size (optional, default: `10`)

> **Important**: Internal transaction tracking is **only supported for EVM chains**. Setting `INTERNAL_TX=true` for Move or Wasm chains will result in a configuration error. For EVM chains, it's automatically enabled by default as it's essential for EVM analytics.

### Metrics Settings

- `METRICS_ENABLED`: Enable metrics endpoint (optional, default: `false`)
- `METRICS_PATH`: Metrics endpoint path (optional, default: `/metrics`)
- `METRICS_PORT`: Metrics server port (optional, default: `9090`)

### CORS Settings (API)

CORS is disabled by default. Enable it when you need to serve browser-based clients, and adjust the following environment variables as needed:

- `CORS_ENABLED`: Enable/disable CORS middleware (optional, default: `false`)
- `CORS_ALLOW_ORIGINS`: Comma-separated list of allowed origins. Supports `*` (allow all) and subdomain patterns like `*.example.com`. (default: `*`) Examples:
  - `*`
  - `https://app.initia.xyz,https://initia.xyz`
  - `*.initia.xyz`
- `CORS_ALLOW_METHODS`: Comma-separated list of allowed HTTP methods (optional, default: `GET,POST,PUT,DELETE,PATCH,OPTIONS,HEAD`)
- `CORS_ALLOW_HEADERS`: Comma-separated list of allowed request headers (optional, default: `Origin, Content-Type, Accept, Authorization, X-Requested-With`)
- `CORS_ALLOW_CREDENTIALS`: Allow sending cookies and credentials (optional, default: `false`)
- `CORS_EXPOSE_HEADERS`: Comma-separated list of exposeHeaders defines a whitelist headers that clients are allowed to access (optional, default: `""`)
- `CORS_MAX_AGE`: Preflight cache TTL in seconds for `Access-Control-Max-Age` (optional, default: `0`)

Notes:
- Empty `Origin` header (non-browser/same-origin requests) is accepted.
- Wildcard `*` allows any origin. Pattern `*.example.com` matches any subdomain, but not the bare domain `example.com`.

### Indexer Start Height

- `START_HEIGHT`: Optional non-negative integer. If provided, the indexer starts from this height instead of the default discovery behavior. Example: `START_HEIGHT=0` to start from genesis, or `START_HEIGHT=9184` to resume from a specific block.

### Example

```sh
# Required environment variables
export DB_DSN='postgres://user:pass@localhost:5432/rollytics'
export CHAIN_ID='myminitia-1'
export VM_TYPE='evm'
export RPC_URL='http://localhost:26657'
export REST_URL='http://localhost:1317'
export JSON_RPC_URL='http://localhost:8545'  # Required for EVM only

# Optional overrides (showing non-default values)
export LOG_LEVEL='info'
export DB_AUTO_MIGRATE='true'
export MAX_CONCURRENT_REQUESTS='100'
# INTERNAL_TX=true is automatically set for EVM chains

# Start height
export START_HEIGHT=9184

# CORS settings
export CORS_ENABLED=true
export CORS_ALLOW_ORIGINS='https://app.initia.xyz,https://initia.xyz,*.doi.com'
export CORS_ALLOW_METHODS='GET,POST,PUT,DELETE,PATCH,OPTIONS,HEAD'
export CORS_ALLOW_HEADERS='Origin, Content-Type, Accept, Authorization, X-Requested-With'
export CORS_ALLOW_CREDENTIALS=true
export CORS_MAX_AGE=300

./rollytics api/indexer
```

#### VM-Specific Defaults

Different VM types have different default configurations:

```sh
# EVM chains - INTERNAL_TX automatically enabled
export VM_TYPE='evm'
# INTERNAL_TX=true (automatic)

# Move chains - INTERNAL_TX disabled by default
export VM_TYPE='move'
# INTERNAL_TX=false (default)

# Wasm chains - INTERNAL_TX disabled by default
export VM_TYPE='wasm'
# INTERNAL_TX=false (default)
```

To explicitly override the VM-specific defaults:

```sh
# Force disable internal TX for EVM chains
export VM_TYPE='evm'
export INTERNAL_TX='false'

# Attempting to enable internal TX for Move/Wasm chains will fail
# export VM_TYPE='move'
# export INTERNAL_TX='true'  # ERROR: Not supported for non-EVM chains
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
  -e DB_DSN="$DB_DSN" \
  -e CHAIN_ID="$CHAIN_ID" \
  -e VM_TYPE="$VM_TYPE" \
  -e RPC_URL="$RPC_URL" \
  -e REST_URL="$REST_URL" \
  -e JSON_RPC_URL="$JSON_RPC_URL" \  # EVM only
  ghcr.io/initia-labs/rollytics:latest api

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
  -e DB_DSN="$DB_DSN" \
  -e CHAIN_ID="$CHAIN_ID" \
  -e VM_TYPE="$VM_TYPE" \
  -e RPC_URL="$RPC_URL" \
  -e REST_URL="$REST_URL" \
  -e JSON_RPC_URL="$JSON_RPC_URL" \  # EVM only
  ghcr.io/initia-labs/rollytics:latest indexer

# Check logs
docker logs -f rollytics-indexer
```

## Development

- Run tests: `make test`
- Lint: `make lint`
- Generate Swagger docs: `make swagger-gen`

## License

Business Source License 1.1 - see [LICENSE](LICENSE) file for details.

## Changelog

For full version history, see **[CHANGELOG.md](CHANGELOG.md)**.
