-- Modify "account_tx" table
ALTER TABLE "public"."account_tx" DROP COLUMN "sequence";
-- Modify "evm_account_tx" table
ALTER TABLE "public"."evm_account_tx" DROP COLUMN "sequence";
-- Modify "nft_tx" table
ALTER TABLE "public"."nft_tx" DROP COLUMN "sequence";
