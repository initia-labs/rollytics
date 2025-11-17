-- atlas:txmode none

-- Create composite index for "tx" matching gorm tags in types/table.go

-- (height, sequence DESC)
CREATE INDEX CONCURRENTLY "tx_height_sequence_desc" ON "public"."tx" ("height", "sequence" DESC);