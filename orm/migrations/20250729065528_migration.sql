-- Create index "block_tx_count" to table: "block"
CREATE INDEX "block_tx_count" ON "public"."block" ("tx_count");
-- Create "evm_internal_tx" table
CREATE TABLE "public"."evm_internal_tx" (
  "height" bigint NOT NULL,
  "hash" text NOT NULL,
  "sequence" bigint NULL,
  "index" bigint NOT NULL,
  "type" text NULL,
  "from" text NULL,
  "to" text NULL,
  "input" text NULL,
  "output" text NULL,
  "value" text NULL,
  "gas" text NULL,
  "gas_used" text NULL,
  "account_ids" bigint[] NULL,
  PRIMARY KEY ("height", "hash", "index")
);
-- Create index "evm_internal_tx_account_ids" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_account_ids" ON "public"."evm_internal_tx" USING gin ("account_ids");
-- Create index "evm_internal_tx_from" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_from" ON "public"."evm_internal_tx" ("from");
-- Create index "evm_internal_tx_index" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_index" ON "public"."evm_internal_tx" ("index");
-- Create index "evm_internal_tx_sequence" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_sequence" ON "public"."evm_internal_tx" ("sequence");
-- Create index "evm_internal_tx_sequence_desc" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_sequence_desc" ON "public"."evm_internal_tx" ("sequence" DESC);
-- Create index "evm_internal_tx_to" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_to" ON "public"."evm_internal_tx" ("to");
-- Create index "evm_internal_tx_type" to table: "evm_internal_tx"
CREATE INDEX "evm_internal_tx_type" ON "public"."evm_internal_tx" ("type");
