# Internal Transaction Indexer

## Overview

The Internal Transaction Indexer is responsible for collecting and indexing EVM internal transactions by utilizing the `debug_traceBlockByNumber` RPC method. This package processes transaction traces to capture all internal calls (such as contract-to-contract interactions) that occur during transaction execution.

### Key Features

- Fetches internal transaction data using `debug_traceBlockByNumber` with the `callTracer` tracer
- Processes the hierarchical structure of transactions:
  - **Top-level calls**: The main transaction calls (from externally owned accounts)
  - **Sub-calls**: Nested internal calls that occur during contract execution (e.g., contract A calling contract B)
- Extracts and indexes addresses involved in internal transactions

### Transaction Structure Example

The `debug_traceBlockByNumber` returns a hierarchical structure:
```json
{
  "result": [
    {
      "txHash": "0x...",
      "result": {
        "type": "CALL",
        "from": "0x...",
        "to": "0x...",
        "value": "0x0",
        "calls": [
          {
            "type": "CALL",
            "from": "0x...",
            "to": "0x...",
            "value": "0x0"
          }
        ]
      }
    }
  ]
}
```

Each transaction can contain multiple levels of internal calls, all of which are captured and indexed.
