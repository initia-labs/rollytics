# Patch v1.0.15: Reset EVM Return Value Cleanup Status

## Purpose

This patch deletes all existing data from the EVM return value cleanup status table to ensure a clean state for the cleanup process.

## What It Does

- Deletes all rows from the `evm_ret_cleanup_status` table

## Why This Is Needed

The EVM return value cleanup extension tracks its progress in the `evm_ret_cleanup_status` table. This patch clears all status entries to:

1. Reset the cleanup tracking state
2. Allow the cleanup process to restart from a fresh state
3. Ensure consistency after modifications to the cleanup logic

## Impact

- All existing EVM return value cleanup status data will be removed
- The cleanup extension will re-initialize its tracking from scratch after this patch is applied
- This is a destructive operation for the status table only (does not affect the actual EVM transaction data)

## Related Changes

- Related to the EVM return value cleanup extension in `indexer/extension/evmret/`
- The cleanup process uses the `evm_ret_cleanup_status` table to track its progress across EVM transactions
