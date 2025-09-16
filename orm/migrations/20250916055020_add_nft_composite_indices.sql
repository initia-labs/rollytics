-- Create index "idx_nft_height_token_id" to table: "nft"
CREATE INDEX "idx_nft_height_token_id" ON "nft" ("height", "token_id");
-- Create index "idx_nft_token_id_height" to table: "nft"
CREATE INDEX "idx_nft_token_id_height" ON "nft" ("token_id", "height");
