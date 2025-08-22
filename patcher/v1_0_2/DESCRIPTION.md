[# Patch v1.0.2 - NFT Data Recovery](https://github.com/initia-labs/rollytics/pull/36)

## Overview
This patch fixes issues in NFT data processing for both Move and Wasm NFTs, and implements a data recovery mechanism to rebuild the NFT table with correct data.

## Bug Fixes

### Move NFT Issues
- **Burn/Transfer/Update Operations**: Fixed operations that were not applied correctly due to incorrect data type handling
- **Data Type Fix**: Changed from `[]string` to `[][]byte` for proper address processing
- **NFT Address Handling**: Fixed NFT address storage to use proper byte format instead of string format

### Wasm NFT Issues  
- **Data Type Fix**: Changed from `[]string` to `[][]byte` to match database schema
- **NFT Transaction Storage**: Fixed issue where NFT transactions failed to store correctly
  - Problem: `GetOrCreateNftIds` returned a map with hex format collection addresses, but lookups were attempted using accAddress format
  - Solution: Corrected the logic to ensure consistent address format (hex) for both storage and lookup

## Patch Implementation

1. **Clear NFT Table**: Removes all existing NFT records (collections remain untouched)
2. **Scan Transactions**: Processes all transactions sequentially by sequence number
   move: scan transactions from nft transaction related account
   wasm: scan all of transactions that have wasm events  
3. **Rebuild NFT State**: 
   - Processes mint events to create NFT records
   - Applies transfer events to update ownership
   - Processes mutation events to update URIs
   - Applies burn events to remove NFTs
4. **Update Transaction References**: Links NFTs to their related transactions

### Database Impact
- **Tables Modified**: `nft` table only
