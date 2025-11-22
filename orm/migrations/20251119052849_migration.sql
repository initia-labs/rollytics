-- Modify "rich_list_status" table
ALTER TABLE "public"."rich_list_status" DROP CONSTRAINT "rich_list_status_pkey", ALTER COLUMN "height" DROP NOT NULL;
-- Create "evm_ret_cleanup_status" table
CREATE TABLE "public"."evm_ret_cleanup_status" (
  "last_cleaned_height" bigint NULL,
  "corrected_records" bigint NULL
);
