-- Create index "evm_tx_signer" to table: "evm_tx"
CREATE INDEX "evm_tx_signer" ON "public"."evm_tx" ("signer");
-- Create index "tx_signer" to table: "tx"
CREATE INDEX "tx_signer" ON "public"."tx" ("signer");
