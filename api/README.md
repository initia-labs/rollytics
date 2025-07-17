# Rollytics REST API

Rollytics offers a unified, read-only HTTP/JSON API for retrieving block, transaction, NFT, and other on-chain data from multi-VM Initia chains.

### Pagination

| Query param | Type   | Default | Notes                                    |
| ----------- | ------ | ------- | ---------------------------------------- |
| `limit`     | number |  `100`  | Max 200                                  |
| `page_key`  | string | —       | Cursor returned in `pagination.next_key` |


## Endpoint

| Method | Path                                                     | Description                               |
| ------ | -------------------------------------------------------- | ----------------------------------------- |
| GET    | `/status`                                                | Indexer status (chain ID, latest height). |
| GET    | `/indexer/block/v1/blocks`                               | List blocks.                              |
| GET    | `/indexer/block/v1/blocks/{height}`                      | Block by height.                          |
| GET    | `/indexer/block/v1/avg_blocktime`                        | Average block time (rolling).             |
| GET    | `/indexer/tx/v1/txs`                                     | List transactions.                        |
| GET    | `/indexer/tx/v1/txs/by_account/{account}`                | Txs for account.                          |
| GET    | `/indexer/tx/v1/txs/by_height/{height}`                  | Txs in block height.                      |
| GET    | `/indexer/tx/v1/txs/{tx_hash}`                           | Tx by hash.                               |
| GET    | `/indexer/tx/v1/evm-txs`                                 | List EVM txs.                             |
| GET    | `/indexer/tx/v1/evm-txs/by_account/{account}`            | EVM txs for account.                      |
| GET    | `/indexer/tx/v1/evm-txs/by_height/{height}`              | EVM txs in block.                         |
| GET    | `/indexer/tx/v1/evm-txs/{tx_hash}`                       | EVM tx by hash.                           |
| GET    | `/indexer/nft/v1/collections`                            | List NFT collections.                     |
| GET    | `/indexer/nft/v1/collections/by_account/{account}`       | Collections owned by account.             |
| GET    | `/indexer/nft/v1/collections/by_name/{name}`             | Collections by name.                      |
| GET    | `/indexer/nft/v1/collections/{collection_addr}`          | Collection by address.                    |
| GET    | `/indexer/nft/v1/tokens/by_account/{account}`            | NFT tokens owned by account.              |
| GET    | `/indexer/nft/v1/tokens/by_collection/{collection_addr}` | Tokens in collection.                     |
| GET    | `/indexer/nft/v1/txs/{collection_addr}/{token_id}`       | NFT transfer history for token.           |

---

## Changelog
For full version history, see **[CHANGELOG.md](../CHANGELOG.md)**.
