# Patch v1.0.12 - SG721 NFT Contract Support

## Overview

This patch adds comprehensive support for SG721 NFT contracts in the wasm-nft indexer, fixing issues with contract querying, event validation, and data processing specific to SG721 implementations.

## Bug Fixes

### SG721 Contract Issues

- **Query Parsing Errors**: Fixed parsing errors when querying SG721 contract info

  - Problem: SG721 contracts use different query message formats that caused parsing failures
  - Solution: Added SG721-specific query handling with fallback mechanisms

- **Event Validation**: Enhanced event validation to properly handle SG721-specific attributes

  - Problem: SG721 contracts may emit events with different attribute patterns
  - Solution: Updated `isValidCollectionEvent` to recognize SG721 event patterns

- **Contract Info Retrieval**: Improved contract info querying for SG721 contracts

  - Problem: SG721 contracts may have different response structures for contract info queries
  - Solution: Added SG721-specific response parsing with proper error handling

- **Address Format Consistency**: Fixed address format handling for SG721 contracts
  - Problem: SG721 contracts may use different address formats in events
  - Solution: Enhanced address conversion logic to handle various SG721 address formats

## Patch Implementation

1. **Enhanced Query Handling**: Added SG721-specific query message support
2. **Improved Event Processing**: Updated event validation and processing for SG721 patterns
3. **Better Error Handling**: Added comprehensive error handling for SG721 contract interactions
4. **Address Format Support**: Enhanced address format conversion for SG721 compatibility

### Database Impact

- **Tables Modified**: No direct database schema changes
- **Data Processing**: Improved handling of SG721 NFT data during indexing
- **Error Reduction**: Reduced failed contract queries and processing errors for SG721 contracts
