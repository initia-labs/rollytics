-- Create "upgrade_history" table
CREATE TABLE "public"."upgrade_history" (
  "version" text NOT NULL,
  "applied" timestamptz NULL,
  PRIMARY KEY ("version")
);
