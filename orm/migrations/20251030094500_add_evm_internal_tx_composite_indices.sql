
-- Create composite indices for "evm_internal_tx" matching gorm tags in types/table.go

-- (height, sequence DESC)
CREATE INDEX "evm_internal_tx_height_sequence_desc"
ON "public"."evm_internal_tx" ("height", "sequence" DESC);

-- (hash_id, sequence DESC)
CREATE INDEX "evm_internal_tx_hash_sequence_desc"
ON "public"."evm_internal_tx" ("hash_id", "sequence" DESC);