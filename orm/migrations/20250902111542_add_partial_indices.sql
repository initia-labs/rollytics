-- Create index "evm_internal_tx_account_sequence_partial" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_account_sequence_partial" ON "public"."evm_internal_tx" ("sequence" DESC) WHERE (account_ids IS NOT NULL);
-- Create index "evm_tx_account_sequence_partial" to table: "evm_tx"
CREATE INDEX "evm_tx_account_sequence_partial" ON "public"."evm_tx" ("sequence" DESC) WHERE (account_ids IS NOT NULL);
-- Create index "tx_account_sequence_partial" to table: "tx"
CREATE INDEX "tx_account_sequence_partial" ON "public"."tx" ("sequence" DESC) WHERE (account_ids IS NOT NULL);
-- Create index "tx_msg_type_sequence_partial" to table: "tx"
CREATE INDEX "tx_msg_type_sequence_partial" ON "public"."tx" ("sequence" DESC) WHERE (msg_type_ids IS NOT NULL);
-- Create index "tx_nft_sequence_partial" to table: "tx"
CREATE INDEX "tx_nft_sequence_partial" ON "public"."tx" ("sequence" DESC) WHERE (nft_ids IS NOT NULL);
