-- Create "account_dict" table
CREATE TABLE "public"."account_dict" (
  "id" bigserial NOT NULL,
  "account" bytea NULL,
  PRIMARY KEY ("id")
);
-- Create index "account_dict_account" to table: "account_dict"
CREATE UNIQUE INDEX "account_dict_account" ON "public"."account_dict" ("account");
-- Create "block" table
CREATE TABLE "public"."block" (
  "chain_id" text NOT NULL,
  "height" bigint NOT NULL,
  "hash" bytea NULL,
  "timestamp" timestamptz NULL,
  "block_time" bigint NULL,
  "proposer" text NULL,
  "gas_used" bigint NULL,
  "gas_wanted" bigint NULL,
  "tx_count" smallint NULL,
  "total_fee" jsonb NULL,
  PRIMARY KEY ("chain_id", "height")
);
-- Create index "block_height_desc" to table: "block"
CREATE INDEX "block_height_desc" ON "public"."block" ("height" DESC);
-- Create index "block_timestamp_desc" to table: "block"
CREATE INDEX "block_timestamp_desc" ON "public"."block" ("timestamp" DESC);
-- Create index "block_tx_count" to table: "block"
CREATE INDEX "block_tx_count" ON "public"."block" ("tx_count");
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
-- Create index "evm_internal_tx_account_ids" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_account_ids" ON "public"."evm_internal_tx" USING gin ("account_ids");
-- Create index "evm_internal_tx_from_id" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_from_id" ON "public"."evm_internal_tx" ("from_id");
-- Create index "evm_internal_tx_index" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_index" ON "public"."evm_internal_tx" ("index");
-- Create index "evm_internal_tx_parent_index" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_parent_index" ON "public"."evm_internal_tx" ("parent_index");
-- Create index "evm_internal_tx_sequence_desc" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_sequence_desc" ON "public"."evm_internal_tx" ("sequence" DESC);
-- Create index "evm_internal_tx_to_id" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_to_id" ON "public"."evm_internal_tx" ("to_id");
-- Create index "evm_internal_tx_type" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_type" ON "public"."evm_internal_tx" ("type");
-- Create "evm_tx" table
CREATE TABLE "public"."evm_tx" (
  "hash" bytea NOT NULL,
  "height" bigint NOT NULL,
  "sequence" bigint NULL,
  "signer_id" bigint NULL,
  "data" jsonb NULL,
  "account_ids" bigint[] NULL,
  PRIMARY KEY ("hash", "height")
);
-- Create index "evm_tx_account_ids" to table: "evm_tx"
CREATE INDEX "evm_tx_account_ids" ON "public"."evm_tx" USING gin ("account_ids");
-- Create index "evm_tx_height" to table: "evm_tx"
CREATE INDEX "evm_tx_height" ON "public"."evm_tx" ("height");
-- Create index "evm_tx_sequence_desc" to table: "evm_tx"
CREATE INDEX "evm_tx_sequence_desc" ON "public"."evm_tx" ("sequence" DESC);
-- Create index "evm_tx_signer_id" to table: "evm_tx"
CREATE INDEX "evm_tx_signer_id" ON "public"."evm_tx" ("signer_id");
-- Create "evm_tx_hash_dict" table
CREATE TABLE "public"."evm_tx_hash_dict" (
  "id" bigserial NOT NULL,
  "hash" bytea NULL,
  PRIMARY KEY ("id")
);
-- Create index "evm_tx_hash_dict_hash" to table: "evm_tx_hash_dict"
CREATE UNIQUE INDEX "evm_tx_hash_dict_hash" ON "public"."evm_tx_hash_dict" ("hash");
-- Create "fa_store" table
CREATE TABLE "public"."fa_store" (
  "store_addr" bytea NOT NULL,
  "owner" bytea NULL,
  PRIMARY KEY ("store_addr")
);
-- Create index "fa_store_owner" to table: "fa_store"
CREATE INDEX "fa_store_owner" ON "public"."fa_store" USING hash ("owner");
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
  "collection_addr" bytea NOT NULL,
  "token_id" text NOT NULL,
  "addr" bytea NULL,
  "height" bigint NULL,
  "timestamp" timestamptz NULL,
  "owner_id" bigint NULL,
  "uri" text NULL,
  PRIMARY KEY ("collection_addr", "token_id")
);
-- Create index "nft_addr" to table: "nft"
CREATE INDEX "nft_addr" ON "public"."nft" USING hash ("addr");
-- Create index "nft_height" to table: "nft"
CREATE INDEX "nft_height" ON "public"."nft" ("height");
-- Create index "nft_owner_id" to table: "nft"
CREATE INDEX "nft_owner_id" ON "public"."nft" ("owner_id");
-- Create index "nft_token_id" to table: "nft"
CREATE INDEX "nft_token_id" ON "public"."nft" ("token_id");
-- Create "nft_collection" table
CREATE TABLE "public"."nft_collection" (
  "addr" bytea NOT NULL,
  "height" bigint NULL,
  "timestamp" timestamptz NULL,
  "name" text NULL,
  "origin_name" text NULL,
  "creator_id" bigint NULL,
  "nft_count" bigint NULL,
  PRIMARY KEY ("addr")
);
-- Create index "nft_collection_creator_id" to table: "nft_collection"
CREATE INDEX "nft_collection_creator_id" ON "public"."nft_collection" ("creator_id");
-- Create index "nft_collection_height" to table: "nft_collection"
CREATE INDEX "nft_collection_height" ON "public"."nft_collection" ("height");
-- Create index "nft_collection_name" to table: "nft_collection"
CREATE INDEX "nft_collection_name" ON "public"."nft_collection" ("name");
-- Create index "nft_collection_origin_name" to table: "nft_collection"
CREATE INDEX "nft_collection_origin_name" ON "public"."nft_collection" ("origin_name");
-- Create "nft_dict" table
CREATE TABLE "public"."nft_dict" (
  "id" bigserial NOT NULL,
  "collection_addr" bytea NULL,
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
  "hash" bytea NOT NULL,
  "height" bigint NOT NULL,
  "sequence" bigint NULL,
  "signer_id" bigint NULL,
  "data" jsonb NULL,
  "account_ids" bigint[] NULL,
  "nft_ids" bigint[] NULL,
  "msg_type_ids" bigint[] NULL,
  "type_tag_ids" bigint[] NULL,
  PRIMARY KEY ("hash", "height")
);
-- Create index "tx_account_ids" to table: "tx"
CREATE INDEX "tx_account_ids" ON "public"."tx" USING gin ("account_ids");
-- Create index "tx_height" to table: "tx"
CREATE INDEX "tx_height" ON "public"."tx" ("height");
-- Create index "tx_msg_type_ids" to table: "tx"
CREATE INDEX "tx_msg_type_ids" ON "public"."tx" USING gin ("msg_type_ids");
-- Create index "tx_nft_ids" to table: "tx"
CREATE INDEX "tx_nft_ids" ON "public"."tx" USING gin ("nft_ids");
-- Create index "tx_sequence_desc" to table: "tx"
CREATE INDEX "tx_sequence_desc" ON "public"."tx" ("sequence" DESC);
-- Create index "tx_signer_id" to table: "tx"
CREATE INDEX "tx_signer_id" ON "public"."tx" ("signer_id");
-- Create index "tx_type_tag_ids" to table: "tx"
CREATE INDEX "tx_type_tag_ids" ON "public"."tx" USING gin ("type_tag_ids");
-- Create "type_tag_dict" table
CREATE TABLE "public"."type_tag_dict" (
  "id" bigserial NOT NULL,
  "type_tag" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "type_tag_dict_type_tag" to table: "type_tag_dict"
CREATE UNIQUE INDEX "type_tag_dict_type_tag" ON "public"."type_tag_dict" ("type_tag");
