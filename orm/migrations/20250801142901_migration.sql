-- Modify "block" table
-- Drop indexes that will be removed
DROP INDEX "public"."block_height";
DROP INDEX "public"."block_timestamp";
-- Alter column types and add new index
ALTER TABLE "public"."block" ALTER COLUMN "hash" TYPE bytea USING decode(hash, 'hex');
ALTER TABLE "public"."block" ALTER COLUMN "tx_count" TYPE smallint;
CREATE INDEX "block_tx_count" ON "public"."block" ("tx_count");

-- Modify "tx" table
-- Drop indexes that will be changed
DROP INDEX "public"."tx_sequence";
DROP INDEX "public"."tx_signer";
-- Alter columns
ALTER TABLE "public"."tx" ALTER COLUMN "hash" TYPE bytea USING decode(hash, 'hex');
ALTER TABLE "public"."tx" DROP COLUMN "signer";
ALTER TABLE "public"."tx" ADD COLUMN "signer_id" bigint NULL;
-- Create new index
CREATE INDEX "tx_signer_id" ON "public"."tx" ("signer_id");

-- Modify "evm_tx" table
-- Drop indexes that will be changed
DROP INDEX "public"."evm_tx_sequence";
DROP INDEX "public"."evm_tx_signer";
-- Alter columns
ALTER TABLE "public"."evm_tx" ALTER COLUMN "hash" TYPE bytea USING decode(hash, 'hex');
ALTER TABLE "public"."evm_tx" DROP COLUMN "signer";
ALTER TABLE "public"."evm_tx" ADD COLUMN "signer_id" bigint NULL;
-- Create new index
CREATE INDEX "evm_tx_signer_id" ON "public"."evm_tx" ("signer_id");

-- Modify "nft_collection" table
ALTER TABLE "public"."nft_collection" ALTER COLUMN "addr" TYPE bytea USING decode(addr, 'hex');
ALTER TABLE "public"."nft_collection" DROP COLUMN "creator";
ALTER TABLE "public"."nft_collection" ADD COLUMN "creator_id" bigint NULL;
-- Create new index
CREATE INDEX "nft_collection_creator_id" ON "public"."nft_collection" ("creator_id");

-- Modify "nft" table
-- Drop index to recreate with different type
DROP INDEX "public"."nft_addr";
DROP INDEX "public"."nft_owner";
-- Alter columns
ALTER TABLE "public"."nft" ALTER COLUMN "collection_addr" TYPE bytea USING decode(collection_addr, 'hex');
ALTER TABLE "public"."nft" ALTER COLUMN "addr" TYPE bytea USING decode(addr, 'hex');
ALTER TABLE "public"."nft" DROP COLUMN "owner";
ALTER TABLE "public"."nft" ADD COLUMN "owner_id" bigint NULL;
-- Create new indexes
CREATE INDEX "nft_addr" ON "public"."nft" USING hash ("addr");
CREATE INDEX "nft_owner_id" ON "public"."nft" ("owner_id");

-- Modify "fa_store" table
-- Drop index to recreate with different type
DROP INDEX "public"."fa_store_owner";
-- Alter columns
ALTER TABLE "public"."fa_store" ALTER COLUMN "store_addr" TYPE bytea USING decode(store_addr, 'hex');
ALTER TABLE "public"."fa_store" ALTER COLUMN "owner" TYPE bytea USING decode(owner, 'hex');
-- Create new index with hash type
CREATE INDEX "fa_store_owner" ON "public"."fa_store" USING hash ("owner");

-- Modify "account_dict" table
ALTER TABLE "public"."account_dict" ALTER COLUMN "account" TYPE bytea USING decode(account, 'hex');

-- Modify "nft_dict" table
ALTER TABLE "public"."nft_dict" ALTER COLUMN "collection_addr" TYPE bytea USING decode(collection_addr, 'hex');

-- Create "evm_tx_hash_dict" table
CREATE TABLE "public"."evm_tx_hash_dict" (
  "id" bigserial NOT NULL,
  "hash" bytea NULL,
  PRIMARY KEY ("id")
);
-- Create unique index
CREATE UNIQUE INDEX "evm_tx_hash_dict_hash" ON "public"."evm_tx_hash_dict" ("hash");

-- Create "evm_internal_tx" table
CREATE TABLE "public"."evm_internal_tx" (
  "height" bigint NOT NULL,
  "hash_id" bigint NOT NULL,
  "index" bigint NOT NULL,
  "parent_index" bigint NULL,
  "sequence" bigint NULL,
  "type" text NULL,
  "from_id" bigint NULL,
  "to_id" bigint NULL,
  "input" bytea NULL,
  "output" bytea NULL,
  "value" bytea NULL,
  "gas" bytea NULL,
  "gas_used" bytea NULL,
  "account_ids" bigint[] NULL,
  PRIMARY KEY ("height", "hash_id", "index")
);
-- Create indexes
CREATE INDEX "evm_internal_tx_index" ON "public"."evm_internal_tx" ("index");
CREATE INDEX "evm_internal_tx_parent_index" ON "public"."evm_internal_tx" ("parent_index");
CREATE INDEX "evm_internal_tx_sequence_desc" ON "public"."evm_internal_tx" ("sequence" DESC);
CREATE INDEX "evm_internal_tx_type" ON "public"."evm_internal_tx" ("type");
CREATE INDEX "evm_internal_tx_from_id" ON "public"."evm_internal_tx" ("from_id");
CREATE INDEX "evm_internal_tx_to_id" ON "public"."evm_internal_tx" ("to_id");
CREATE INDEX "evm_internal_tx_account_ids" ON "public"."evm_internal_tx" USING gin ("account_ids");