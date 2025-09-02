-- Optimize array-based queries with COUNT() operations
-- Addresses slow pagination performance when limit >= 2 across multiple tables
-- Issue: GIN index scans followed by expensive sorting and counting operations

-- 1. TX table: account_ids array queries (main issue)
-- Used by: /indexer/tx/v1/txs/by_account/{account}
CREATE INDEX CONCURRENTLY "tx_account_sequence_partial"
ON "public"."tx" ("sequence" DESC)
WHERE account_ids IS NOT NULL;

-- 2. TX table: nft_ids array queries  
-- Used by: /indexer/nft/v1/txs/{collection_addr}/{token_id} (WasmVM/EVM cases)
CREATE INDEX CONCURRENTLY "tx_nft_sequence_partial"
ON "public"."tx" ("sequence" DESC)
WHERE nft_ids IS NOT NULL;

-- 3. TX table: msg_type_ids array queries
-- Used by: /indexer/tx/v1/txs?msgs=... (message type filtering)
CREATE INDEX CONCURRENTLY "tx_msg_type_sequence_partial"
ON "public"."tx" ("sequence" DESC)
WHERE msg_type_ids IS NOT NULL;

-- 4. EVM TX table: account_ids array queries
-- Used by: /indexer/tx/v1/evm-txs/by_account/{account}
CREATE INDEX CONCURRENTLY "evm_tx_account_sequence_partial"
ON "public"."evm_tx" ("sequence" DESC)
WHERE account_ids IS NOT NULL;

-- 5. EVM Internal TX table: account_ids array queries
-- Used by: /indexer/tx/v1/evm-internal-txs/by_account/{account}
CREATE INDEX CONCURRENTLY "evm_internal_tx_account_sequence_partial"
ON "public"."evm_internal_tx" ("sequence" DESC)
WHERE account_ids IS NOT NULL;

-- Performance benefit explanation:
-- - Reduces index size by excluding irrelevant rows
-- - Enables PostgreSQL to skip expensive sorting operations
-- - Dramatically improves COUNT() performance for pagination
-- - Maintains existing GIN indexes for array intersection operations