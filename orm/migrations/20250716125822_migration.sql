-- Create index "evm_tx_account_ids" to table: "evm_tx"
CREATE INDEX "evm_tx_account_ids" ON "public"."evm_tx" USING gin ("account_ids");
-- Create index "tx_account_ids" to table: "tx"
CREATE INDEX "tx_account_ids" ON "public"."tx" USING gin ("account_ids");
-- Create index "tx_msg_type_ids" to table: "tx"
CREATE INDEX "tx_msg_type_ids" ON "public"."tx" USING gin ("msg_type_ids");
-- Create index "tx_nft_ids" to table: "tx"
CREATE INDEX "tx_nft_ids" ON "public"."tx" USING gin ("nft_ids");
-- Create index "tx_type_tag_ids" to table: "tx"
CREATE INDEX "tx_type_tag_ids" ON "public"."tx" USING gin ("type_tag_ids");
