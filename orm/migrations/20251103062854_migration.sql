-- Create "rich_list" table
CREATE TABLE "public"."rich_list" (
  "id" bigserial NOT NULL,
  "denom" text NOT NULL,
  "amount" numeric NULL,
  PRIMARY KEY ("id", "denom")
);
-- Create "rich_list_status" table
CREATE TABLE "public"."rich_list_status" (
  "height" bigserial NOT NULL,
  PRIMARY KEY ("height")
);
