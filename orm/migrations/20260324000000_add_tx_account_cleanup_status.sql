-- Create "tx_account_cleanup_status" table
CREATE TABLE "public"."tx_account_cleanup_status" (
  "last_cleaned_sequence" bigint NULL,
  "deleted_records" bigint NULL,
  "inserted_records" bigint NULL
);
