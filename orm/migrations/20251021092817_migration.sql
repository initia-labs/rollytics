-- Drop index "evm_internal_tx_account_sequence_partial" from table: "evm_internal_tx"
DROP INDEX "public"."evm_internal_tx_account_sequence_partial";
-- Modify "evm_internal_tx" table
ALTER TABLE "public"."evm_internal_tx" DROP COLUMN "account_ids";
-- Create index "evm_internal_tx_account_sequence_partial" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_account_sequence_partial" ON "public"."evm_internal_tx" ("sequence" DESC);
-- Drop index "evm_internal_tx_accounts_account_sequence_desc" from table: "evm_internal_tx_accounts"
DROP INDEX "public"."evm_internal_tx_accounts_account_sequence_desc";
-- Drop index "evm_internal_tx_accounts_sequence_idx" from table: "evm_internal_tx_accounts"
DROP INDEX "public"."evm_internal_tx_accounts_sequence_idx";
-- Drop index "evm_tx_account_sequence_partial" from table: "evm_tx"
DROP INDEX "public"."evm_tx_account_sequence_partial";
-- Modify "evm_tx" table
ALTER TABLE "public"."evm_tx" DROP COLUMN "account_ids";
-- Create index "evm_tx_account_sequence_partial" to table: "evm_tx"
CREATE INDEX "evm_tx_account_sequence_partial" ON "public"."evm_tx" ("sequence" DESC);
-- Drop index "evm_tx_accounts_account_sequence_desc" from table: "evm_tx_accounts"
DROP INDEX "public"."evm_tx_accounts_account_sequence_desc";
-- Drop index "evm_tx_accounts_sequence_idx" from table: "evm_tx_accounts"
DROP INDEX "public"."evm_tx_accounts_sequence_idx";
-- Drop index "evm_tx_accounts_signer_sequence_desc" from table: "evm_tx_accounts"
DROP INDEX "public"."evm_tx_accounts_signer_sequence_desc";
-- Modify "evm_tx_accounts" table
ALTER TABLE "public"."evm_tx_accounts" ALTER COLUMN "signer" DROP NOT NULL, ALTER COLUMN "signer" DROP DEFAULT;
-- Drop index "tx_account_sequence_partial" from table: "tx"
DROP INDEX "public"."tx_account_sequence_partial";
-- Drop index "tx_msg_type_sequence_partial" from table: "tx"
DROP INDEX "public"."tx_msg_type_sequence_partial";
-- Drop index "tx_nft_sequence_partial" from table: "tx"
DROP INDEX "public"."tx_nft_sequence_partial";
-- Modify "tx" table
ALTER TABLE "public"."tx" DROP COLUMN "account_ids", DROP COLUMN "nft_ids", DROP COLUMN "msg_type_ids", DROP COLUMN "type_tag_ids";
-- Create index "tx_account_sequence_partial" to table: "tx"
CREATE INDEX "tx_account_sequence_partial" ON "public"."tx" ("sequence" DESC);
-- Create index "tx_msg_type_sequence_partial" to table: "tx"
CREATE INDEX "tx_msg_type_sequence_partial" ON "public"."tx" ("sequence" DESC);
-- Create index "tx_nft_sequence_partial" to table: "tx"
CREATE INDEX "tx_nft_sequence_partial" ON "public"."tx" ("sequence" DESC);
-- Drop index "tx_accounts_account_sequence_desc" from table: "tx_accounts"
DROP INDEX "public"."tx_accounts_account_sequence_desc";
-- Drop index "tx_accounts_sequence_idx" from table: "tx_accounts"
DROP INDEX "public"."tx_accounts_sequence_idx";
-- Drop index "tx_accounts_signer_sequence_desc" from table: "tx_accounts"
DROP INDEX "public"."tx_accounts_signer_sequence_desc";
-- Modify "tx_accounts" table
ALTER TABLE "public"."tx_accounts" ALTER COLUMN "signer" DROP NOT NULL, ALTER COLUMN "signer" DROP DEFAULT;
-- Drop index "tx_msg_types_sequence_desc" from table: "tx_msg_types"
DROP INDEX "public"."tx_msg_types_sequence_desc";
-- Drop index "tx_msg_types_sequence_idx" from table: "tx_msg_types"
DROP INDEX "public"."tx_msg_types_sequence_idx";
-- Drop index "tx_nfts_sequence_desc" from table: "tx_nfts"
DROP INDEX "public"."tx_nfts_sequence_desc";
-- Drop index "tx_nfts_sequence_idx" from table: "tx_nfts"
DROP INDEX "public"."tx_nfts_sequence_idx";
-- Drop index "tx_type_tags_sequence_desc" from table: "tx_type_tags"
DROP INDEX "public"."tx_type_tags_sequence_desc";
-- Drop index "tx_type_tags_sequence_idx" from table: "tx_type_tags"
DROP INDEX "public"."tx_type_tags_sequence_idx";
