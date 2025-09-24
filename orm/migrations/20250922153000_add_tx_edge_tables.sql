-- Introduce sequence-based edge tables to optimize account, NFT, message-type, and type-tag queries
CREATE TABLE
  "public"."tx_accounts" (
    "account_id" bigint NOT NULL,
    "sequence" bigint NOT NULL,
    "signer" boolean NOT NULL DEFAULT FALSE,
    PRIMARY KEY ("account_id", "sequence")
  );

CREATE INDEX "tx_accounts_sequence_idx" ON "public"."tx_accounts" ("sequence");
CREATE INDEX "tx_accounts_account_sequence_desc" ON "public"."tx_accounts" ("account_id", "sequence" DESC);
CREATE INDEX "tx_accounts_signer_sequence_desc" ON "public"."tx_accounts" ("account_id", "signer", "sequence" DESC);

CREATE TABLE
  "public"."tx_nfts" (
    "nft_id" bigint NOT NULL,
    "sequence" bigint NOT NULL,
    PRIMARY KEY ("nft_id", "sequence")
  );

CREATE INDEX "tx_nfts_sequence_idx" ON "public"."tx_nfts" ("sequence");
CREATE INDEX "tx_nfts_sequence_desc" ON "public"."tx_nfts" ("nft_id", "sequence" DESC);

CREATE TABLE
  "public"."tx_msg_types" (
    "msg_type_id" bigint NOT NULL,
    "sequence" bigint NOT NULL,
    PRIMARY KEY ("msg_type_id", "sequence")
  );

CREATE INDEX "tx_msg_types_sequence_idx" ON "public"."tx_msg_types" ("sequence");
CREATE INDEX "tx_msg_types_sequence_desc" ON "public"."tx_msg_types" ("msg_type_id", "sequence" DESC);

CREATE TABLE
  "public"."tx_type_tags" (
    "type_tag_id" bigint NOT NULL,
    "sequence" bigint NOT NULL,
    PRIMARY KEY ("type_tag_id", "sequence")
  );

CREATE INDEX "tx_type_tags_sequence_idx" ON "public"."tx_type_tags" ("sequence");
CREATE INDEX "tx_type_tags_sequence_desc" ON "public"."tx_type_tags" ("type_tag_id", "sequence" DESC);

-- NOTE: This is not strictly necessary, but added for symmetry and potential future-proofing
-- ALTER TABLE "public"."tx" ADD CONSTRAINT "tx_sequence_unique" UNIQUE ("sequence");

-- Edge tables for evm tx account relationships
CREATE TABLE
  "public"."evm_tx_accounts" (
    "account_id" bigint NOT NULL,
    "sequence" bigint NOT NULL,
    "signer" boolean NOT NULL DEFAULT FALSE,
    PRIMARY KEY ("account_id", "sequence")
  );

CREATE INDEX "evm_tx_accounts_sequence_idx" ON "public"."evm_tx_accounts" ("sequence");
CREATE INDEX "evm_tx_accounts_account_sequence_desc" ON "public"."evm_tx_accounts" ("account_id", "sequence" DESC);
CREATE INDEX "evm_tx_accounts_signer_sequence_desc" ON "public"."evm_tx_accounts" ("account_id", "signer", "sequence" DESC);

-- NOTE: This is not strictly necessary, but added for symmetry and potential future-proofing
-- ALTER TABLE "public"."evm_tx" ADD CONSTRAINT "evm_tx_sequence_unique" UNIQUE ("sequence");

CREATE TABLE
  "public"."evm_internal_tx_accounts" (
    "account_id" bigint NOT NULL,
    "sequence" bigint NOT NULL,
    PRIMARY KEY ("account_id", "sequence")
  );

CREATE INDEX "evm_internal_tx_accounts_sequence_idx" ON "public"."evm_internal_tx_accounts" ("sequence");
CREATE INDEX "evm_internal_tx_accounts_account_sequence_desc" ON "public"."evm_internal_tx_accounts" ("account_id", "sequence" DESC);

-- NOTE: This is not strictly necessary, but added for symmetry and potential future-proofing
-- ALTER TABLE "public"."evm_internal_tx" ADD CONSTRAINT "evm_internal_tx_sequence_unique" UNIQUE ("sequence");
