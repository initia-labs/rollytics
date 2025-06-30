-- Create "account_tx" table
CREATE TABLE "public"."account_tx" (
  "chain_id" text NOT NULL,
  "hash" text NOT NULL,
  "account" text NOT NULL,
  "height" bigint NOT NULL,
  PRIMARY KEY ("chain_id", "hash", "account", "height")
);
-- Create index "account_tx_account" to table: "account_tx"
CREATE INDEX "account_tx_account" ON "public"."account_tx" ("account");
-- Create index "account_tx_hash" to table: "account_tx"
CREATE INDEX "account_tx_hash" ON "public"."account_tx" ("hash");
-- Create index "account_tx_height" to table: "account_tx"
CREATE INDEX "account_tx_height" ON "public"."account_tx" ("height");
-- Create "block" table
CREATE TABLE "public"."block" (
  "chain_id" text NOT NULL,
  "height" bigint NOT NULL,
  "hash" text NULL,
  "timestamp" timestamptz NULL,
  "block_time" bigint NULL,
  "proposer" text NULL,
  "gas_used" bigint NULL,
  "gas_wanted" bigint NULL,
  "tx_count" bigint NULL,
  "total_fee" jsonb NULL,
  PRIMARY KEY ("chain_id", "height")
);
-- Create index "block_height" to table: "block"
CREATE INDEX "block_height" ON "public"."block" ("height");
-- Create index "block_timestamp" to table: "block"
CREATE INDEX "block_timestamp" ON "public"."block" ("timestamp");
-- Create index "block_timestamp_desc" to table: "block"
CREATE INDEX "block_timestamp_desc" ON "public"."block" ("timestamp" DESC);
-- Create "evm_account_tx" table
CREATE TABLE "public"."evm_account_tx" (
  "chain_id" text NOT NULL,
  "hash" text NOT NULL,
  "account" text NOT NULL,
  "height" bigint NOT NULL,
  PRIMARY KEY ("chain_id", "hash", "account", "height")
);
-- Create index "evm_account_tx_account" to table: "evm_account_tx"
CREATE INDEX "evm_account_tx_account" ON "public"."evm_account_tx" ("account");
-- Create index "evm_account_tx_hash" to table: "evm_account_tx"
CREATE INDEX "evm_account_tx_hash" ON "public"."evm_account_tx" ("hash");
-- Create index "evm_account_tx_height" to table: "evm_account_tx"
CREATE INDEX "evm_account_tx_height" ON "public"."evm_account_tx" ("height");
-- Create "evm_tx" table
CREATE TABLE "public"."evm_tx" (
  "chain_id" text NOT NULL,
  "hash" text NOT NULL,
  "height" bigint NOT NULL,
  "sequence" bigint NULL,
  "signer" text NULL,
  "data" jsonb NULL,
  PRIMARY KEY ("chain_id", "hash", "height")
);
-- Create index "evm_tx_hash" to table: "evm_tx"
CREATE INDEX "evm_tx_hash" ON "public"."evm_tx" ("hash");
-- Create index "evm_tx_height" to table: "evm_tx"
CREATE INDEX "evm_tx_height" ON "public"."evm_tx" ("height");
-- Create index "evm_tx_sequence" to table: "evm_tx"
CREATE INDEX "evm_tx_sequence" ON "public"."evm_tx" ("sequence");
-- Create "fa_store" table
CREATE TABLE "public"."fa_store" (
  "chain_id" text NOT NULL,
  "store_addr" text NOT NULL,
  "owner" text NULL,
  PRIMARY KEY ("chain_id", "store_addr")
);
-- Create index "fa_store_owner" to table: "fa_store"
CREATE INDEX "fa_store_owner" ON "public"."fa_store" ("owner");
-- Create index "fa_store_store_addr" to table: "fa_store"
CREATE INDEX "fa_store_store_addr" ON "public"."fa_store" ("store_addr");
-- Create "msg_type" table
CREATE TABLE "public"."msg_type" (
  "id" bigserial NOT NULL,
  "msg_type" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_msg_type_msg_type" to table: "msg_type"
CREATE UNIQUE INDEX "idx_msg_type_msg_type" ON "public"."msg_type" ("msg_type");
-- Create "nft" table
CREATE TABLE "public"."nft" (
  "chain_id" text NOT NULL,
  "collection_addr" text NOT NULL,
  "token_id" text NOT NULL,
  "addr" text NULL,
  "height" bigint NULL,
  "owner" text NULL,
  "uri" text NULL,
  PRIMARY KEY ("chain_id", "collection_addr", "token_id")
);
-- Create index "nft_addr" to table: "nft"
CREATE INDEX "nft_addr" ON "public"."nft" ("addr");
-- Create index "nft_collection_addr" to table: "nft"
CREATE INDEX "nft_collection_addr" ON "public"."nft" ("collection_addr");
-- Create index "nft_height" to table: "nft"
CREATE INDEX "nft_height" ON "public"."nft" ("height");
-- Create index "nft_owner" to table: "nft"
CREATE INDEX "nft_owner" ON "public"."nft" ("owner");
-- Create index "nft_token_id" to table: "nft"
CREATE INDEX "nft_token_id" ON "public"."nft" ("token_id");
-- Create "nft_collection" table
CREATE TABLE "public"."nft_collection" (
  "chain_id" text NOT NULL,
  "addr" text NOT NULL,
  "height" bigint NULL,
  "name" text NULL,
  "origin_name" text NULL,
  "creator" text NULL,
  "nft_count" bigint NULL,
  PRIMARY KEY ("chain_id", "addr")
);
-- Create index "nft_collection_height" to table: "nft_collection"
CREATE INDEX "nft_collection_height" ON "public"."nft_collection" ("height");
-- Create index "nft_collection_name" to table: "nft_collection"
CREATE INDEX "nft_collection_name" ON "public"."nft_collection" ("name");
-- Create index "nft_collection_origin_name" to table: "nft_collection"
CREATE INDEX "nft_collection_origin_name" ON "public"."nft_collection" ("origin_name");
-- Create "nft_tx" table
CREATE TABLE "public"."nft_tx" (
  "chain_id" text NOT NULL,
  "hash" text NOT NULL,
  "collection_addr" text NOT NULL,
  "token_id" text NOT NULL,
  "height" bigint NOT NULL,
  PRIMARY KEY ("chain_id", "hash", "collection_addr", "token_id", "height")
);
-- Create index "nft_tx_collection_addr" to table: "nft_tx"
CREATE INDEX "nft_tx_collection_addr" ON "public"."nft_tx" ("collection_addr");
-- Create index "nft_tx_hash" to table: "nft_tx"
CREATE INDEX "nft_tx_hash" ON "public"."nft_tx" ("hash");
-- Create index "nft_tx_height" to table: "nft_tx"
CREATE INDEX "nft_tx_height" ON "public"."nft_tx" ("height");
-- Create index "nft_tx_token_id" to table: "nft_tx"
CREATE INDEX "nft_tx_token_id" ON "public"."nft_tx" ("token_id");
-- Create "seq_info" table
CREATE TABLE "public"."seq_info" (
  "chain_id" text NOT NULL,
  "name" text NOT NULL,
  "sequence" bigint NULL,
  PRIMARY KEY ("chain_id", "name")
);
-- Create "tx" table
CREATE TABLE "public"."tx" (
  "chain_id" text NOT NULL,
  "hash" text NOT NULL,
  "height" bigint NOT NULL,
  "sequence" bigint NULL,
  "signer" text NULL,
  "data" jsonb NULL,
  "msg_type_ids" bigint[] NULL,
  "type_tag_ids" bigint[] NULL,
  PRIMARY KEY ("chain_id", "hash", "height")
);
-- Create index "tx_hash" to table: "tx"
CREATE INDEX "tx_hash" ON "public"."tx" ("hash");
-- Create index "tx_height" to table: "tx"
CREATE INDEX "tx_height" ON "public"."tx" ("height");
-- Create index "tx_sequence" to table: "tx"
CREATE INDEX "tx_sequence" ON "public"."tx" ("sequence");
-- Create "type_tag" table
CREATE TABLE "public"."type_tag" (
  "id" bigserial NOT NULL,
  "type_tag" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_type_tag_type_tag" to table: "type_tag"
CREATE UNIQUE INDEX "idx_type_tag_type_tag" ON "public"."type_tag" ("type_tag");
