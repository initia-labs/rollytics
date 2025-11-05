-- Create index "rich_list_denom_amount" to table: "rich_list"
-- This index optimizes queries filtering by denom and ordering by amount DESC
CREATE INDEX "rich_list_denom_amount" ON "public"."rich_list" ("denom", "amount" DESC);
