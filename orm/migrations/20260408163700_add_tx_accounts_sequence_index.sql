-- atlas:txmode none

-- Create index on "sequence" for "tx_accounts" matching gorm tags in types/table.go
CREATE INDEX CONCURRENTLY "idx_tx_accounts_sequence" ON "public"."tx_accounts" ("sequence");
