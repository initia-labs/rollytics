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
  "value" bigint NULL,
  "gas" bigint NULL,
  "gas_used" bigint NULL,
  "account_ids" bigint[] NULL,
  "pre_state" jsonb NULL,
  "post_state" jsonb NULL,
  PRIMARY KEY ("height", "hash", "index")
);
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
