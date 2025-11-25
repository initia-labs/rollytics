# Patch v1.0.14: Reset Rich List Data

## Purpose

This patch deletes all existing data from the rich list tables to ensure a clean state for the EVM rich list implementation.

## What It Does

- Deletes all rows from the `rich_list` table
- Deletes all rows from the `rich_list_status` table

## Why This Is Needed

The rich list extension previously had deletion logic in its `Initialize()` method that ran every time the extension started. This patch moves that logic into a one-time migration to:

1. Ensure the deletion only happens once during the upgrade
2. Clean up existing data that may have been collected incorrectly
3. Allow the extension to start fresh with the corrected transaction-based implementation

## Impact

- All existing rich list balance data will be removed
- The rich list extension will re-index from the latest block height after this patch is applied
- This is a destructive operation but necessary for data consistency

## Related Changes

- The `Initialize()` method in `indexer/extension/richlist/extension.go` no longer performs deletion
- The `Run()` function in `indexer/extension/richlist/evmrichlist/run.go` now wraps all operations in a transaction for consistency
