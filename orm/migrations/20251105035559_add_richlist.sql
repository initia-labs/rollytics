-- Create "rich_list" table
CREATE TABLE "public"."rich_list" (
  "id" bigserial NOT NULL,
  "denom" text NOT NULL,
  "amount" numeric NULL,
  PRIMARY KEY ("id", "denom")
);
-- Create "rich_list_status" table
CREATE TABLE "public"."rich_list_status" (
  "height" bigint NOT NULL,
  PRIMARY KEY ("height")
);

-- Create index "rich_list_denom_amount" to table: "rich_list"
-- This index optimizes queries filtering by denom and ordering by amount DESC
CREATE INDEX "rich_list_denom_amount" ON "public"."rich_list" ("denom", "amount" DESC);
