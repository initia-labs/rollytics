-- Create index "nft_height_token_id" to table: "nft"
CREATE INDEX "nft_height_token_id" ON "public"."nft" ("height", "token_id");
-- Create index "nft_token_id_height" to table: "nft"
CREATE INDEX "nft_token_id_height" ON "public"."nft" ("token_id", "height");
