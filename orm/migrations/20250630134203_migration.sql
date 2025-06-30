-- Modify "account_tx" table
ALTER TABLE "public"."account_tx" ADD COLUMN "sequence" bigint NULL;
-- Create index "account_tx_sequence" to table: "account_tx"
CREATE INDEX "account_tx_sequence" ON "public"."account_tx" ("sequence");
-- Modify "evm_account_tx" table
ALTER TABLE "public"."evm_account_tx" ADD COLUMN "sequence" bigint NULL;
-- Create index "evm_account_tx_sequence" to table: "evm_account_tx"
CREATE INDEX "evm_account_tx_sequence" ON "public"."evm_account_tx" ("sequence");
-- Modify "nft_tx" table
ALTER TABLE "public"."nft_tx" ADD COLUMN "sequence" bigint NULL;
-- Create index "nft_tx_sequence" to table: "nft_tx"
CREATE INDEX "nft_tx_sequence" ON "public"."nft_tx" ("sequence");
