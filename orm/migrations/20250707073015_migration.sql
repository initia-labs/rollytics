-- Create "account_dict" table
CREATE TABLE "public"."account_dict" (
  "id" bigserial NOT NULL,
  "account" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "account_dict_account" to table: "account_dict"
CREATE UNIQUE INDEX "account_dict_account" ON "public"."account_dict" ("account");
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
-- Create index "block_height_desc" to table: "block"
CREATE INDEX "block_height_desc" ON "public"."block" ("height" DESC);
-- Create index "block_timestamp" to table: "block"
CREATE INDEX "block_timestamp" ON "public"."block" ("timestamp");
-- Create index "block_timestamp_desc" to table: "block"
CREATE INDEX "block_timestamp_desc" ON "public"."block" ("timestamp" DESC);
-- Create "evm_tx" table
CREATE TABLE "public"."evm_tx" (
  "hash" text NOT NULL,
  "height" bigint NOT NULL,
  "sequence" bigint NULL,
  "signer" text NULL,
  "data" jsonb NULL,
  "account_ids" bigint[] NULL,
  PRIMARY KEY ("hash", "height")
);
-- Create index "evm_tx_height" to table: "evm_tx"
CREATE INDEX "evm_tx_height" ON "public"."evm_tx" ("height");
-- Create index "evm_tx_sequence" to table: "evm_tx"
CREATE INDEX "evm_tx_sequence" ON "public"."evm_tx" ("sequence");
-- Create index "evm_tx_sequence_desc" to table: "evm_tx"
CREATE INDEX "evm_tx_sequence_desc" ON "public"."evm_tx" ("sequence" DESC);
-- Create index "evm_tx_signer" to table: "evm_tx"
CREATE INDEX "evm_tx_signer" ON "public"."evm_tx" ("signer");
-- Create "fa_store" table
CREATE TABLE "public"."fa_store" (
  "store_addr" text NOT NULL,
  "owner" text NULL,
  PRIMARY KEY ("store_addr")
);
-- Create index "fa_store_owner" to table: "fa_store"
CREATE INDEX "fa_store_owner" ON "public"."fa_store" ("owner");
-- Create "msg_type_dict" table
CREATE TABLE "public"."msg_type_dict" (
  "id" bigserial NOT NULL,
  "msg_type" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "msg_type_dict_msg_type" to table: "msg_type_dict"
CREATE UNIQUE INDEX "msg_type_dict_msg_type" ON "public"."msg_type_dict" ("msg_type");
-- Create "nft" table
CREATE TABLE "public"."nft" (
  "collection_addr" text NOT NULL,
  "token_id" text NOT NULL,
  "addr" text NULL,
  "height" bigint NULL,
  "owner" text NULL,
  "uri" text NULL,
  PRIMARY KEY ("collection_addr", "token_id")
);
-- Create index "nft_addr" to table: "nft"
CREATE INDEX "nft_addr" ON "public"."nft" ("addr");
-- Create index "nft_height" to table: "nft"
CREATE INDEX "nft_height" ON "public"."nft" ("height");
-- Create index "nft_owner" to table: "nft"
CREATE INDEX "nft_owner" ON "public"."nft" ("owner");
-- Create index "nft_token_id" to table: "nft"
CREATE INDEX "nft_token_id" ON "public"."nft" ("token_id");
-- Create "nft_collection" table
CREATE TABLE "public"."nft_collection" (
  "addr" text NOT NULL,
  "height" bigint NULL,
  "name" text NULL,
  "origin_name" text NULL,
  "creator" text NULL,
  "nft_count" bigint NULL,
  PRIMARY KEY ("addr")
);
-- Create index "nft_collection_height" to table: "nft_collection"
CREATE INDEX "nft_collection_height" ON "public"."nft_collection" ("height");
-- Create index "nft_collection_name" to table: "nft_collection"
CREATE INDEX "nft_collection_name" ON "public"."nft_collection" ("name");
-- Create index "nft_collection_origin_name" to table: "nft_collection"
CREATE INDEX "nft_collection_origin_name" ON "public"."nft_collection" ("origin_name");
-- Create "nft_dict" table
CREATE TABLE "public"."nft_dict" (
  "id" bigserial NOT NULL,
  "collection_addr" text NULL,
  "token_id" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "nft_dict_collection_addr_token_id" to table: "nft_dict"
CREATE UNIQUE INDEX "nft_dict_collection_addr_token_id" ON "public"."nft_dict" ("collection_addr", "token_id");
-- Create "seq_info" table
CREATE TABLE "public"."seq_info" (
  "name" text NOT NULL,
  "sequence" bigint NULL,
  PRIMARY KEY ("name")
);
-- Create "tx" table
CREATE TABLE "public"."tx" (
  "hash" text NOT NULL,
  "height" bigint NOT NULL,
  "sequence" bigint NULL,
  "signer" text NULL,
  "data" jsonb NULL,
  "account_ids" bigint[] NULL,
  "nft_ids" bigint[] NULL,
  "msg_type_ids" bigint[] NULL,
  "type_tag_ids" bigint[] NULL,
  PRIMARY KEY ("hash", "height")
);
-- Create index "tx_height" to table: "tx"
CREATE INDEX "tx_height" ON "public"."tx" ("height");
-- Create index "tx_sequence" to table: "tx"
CREATE INDEX "tx_sequence" ON "public"."tx" ("sequence");
-- Create index "tx_sequence_desc" to table: "tx"
CREATE INDEX "tx_sequence_desc" ON "public"."tx" ("sequence" DESC);
-- Create index "tx_signer" to table: "tx"
CREATE INDEX "tx_signer" ON "public"."tx" ("signer");
-- Create "type_tag_dict" table
CREATE TABLE "public"."type_tag_dict" (
  "id" bigserial NOT NULL,
  "type_tag" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "type_tag_dict_type_tag" to table: "type_tag_dict"
CREATE UNIQUE INDEX "type_tag_dict_type_tag" ON "public"."type_tag_dict" ("type_tag");
