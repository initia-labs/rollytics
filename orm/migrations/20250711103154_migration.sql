-- Create "collected_evm_internal_txes" table
CREATE TABLE "public"."collected_evm_internal_txes" (
  "height" bigint NOT NULL,
  "hash" text NOT NULL,
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
-- Create index "evm_internal_tx_from" to table: "collected_evm_internal_txes"
CREATE INDEX "evm_internal_tx_from" ON "public"."collected_evm_internal_txes" ("from");
-- Create index "evm_internal_tx_index" to table: "collected_evm_internal_txes"
CREATE INDEX "evm_internal_tx_index" ON "public"."collected_evm_internal_txes" ("index");
-- Create index "evm_internal_tx_to" to table: "collected_evm_internal_txes"
CREATE INDEX "evm_internal_tx_to" ON "public"."collected_evm_internal_txes" ("to");
-- Create index "evm_internal_tx_type" to table: "collected_evm_internal_txes"
CREATE INDEX "evm_internal_tx_type" ON "public"."collected_evm_internal_txes" ("type");
