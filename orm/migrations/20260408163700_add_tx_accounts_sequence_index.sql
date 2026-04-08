-- atlas:txmode none

-- Create missing sequence indexes used by edge-table sequence scans.
CREATE INDEX CONCURRENTLY "idx_tx_accounts_sequence" ON "public"."tx_accounts" ("sequence");
CREATE INDEX CONCURRENTLY "idx_tx_msg_types_sequence" ON "public"."tx_msg_types" ("sequence");
