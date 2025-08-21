-- Create "patch" table
CREATE TABLE "public"."patch" (
  "version" text NOT NULL,
  "applied" timestamptz NULL,
  PRIMARY KEY ("version")
);
