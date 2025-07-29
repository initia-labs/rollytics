# Indexer

The indexer is responsible for fetching blockchain data and storing it in the database for efficient querying.

## Architecture

```text
┌─────────────────────────────────────────────────────────────────┐
│                           Indexer                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────────┐     │
│  │   Scraper   │  │  Collector  │  │      Extension       │     │
│  │             │  │             │  │                      │     │
│  │ - Fetch     │  │ - Process   │  │  ┌────────────────┐  │     │
│  │   blocks    │  │   blocks    │  │  │  Internal TX   │  │     │
│  │ - Send to   │  │ - Run       │  │  │                │  │     │
│  │  Collector  │  │   submodules│  │  │ - Read from DB │  │     │
│  └──────┬──────┘  │ - Write to  │  │  │ - Process ITX  │  │     │
│         │         │   Database  │  │  │ - Write to DB  │  │     │
│         │         └──────┬──────┘  │  └────────────────┘  │     │
│         │                │         │  ┌────────────────┐  │     │
│         └────────────────▶         │  │ Future Ext...  │  │     │   
│                          │         │  └────────────────┘  │     │
│                     Submodules:    └───────────┬──────────┘     │
│                     - block                    │                │
│                     - tx                       │ Read/Write     │
│                     - evm-nft                  │ Database       │
│                     - move-nft                 │                │
│                     - wasm-nft                 │                │
│                     - nft-pair                 │                │
│                          │                     │                │
│                          ▼                     ▼                │
│    ┌───────────────────────────────────────────────────────┐    │
│    │                                                       │    │
│    │                      Database                         │    │
│    │                                                       │    │
│    └───────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

## Components

- **Scraper**: Fetches blocks from blockchain nodes (RPC/REST/JSON-RPC) and sends them to Collector
- **Collector**: Processes blocks through specialized submodules and writes data to database
- **Extension Manager**: Reads data from database, processes through extensions, and writes results back to database

## Data Flow

1. Scraper fetches blocks from blockchain
2. Scraper sends blocks directly to Collector
3. Collector processes blocks through submodules
4. Collector writes data to database
5. Extensions process data with their specific logic and write results back to database
