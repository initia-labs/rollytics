package evmrichlist

import (
	"encoding/json"
	"log/slog"
	"strings"

	sdkmath "cosmossdk.io/math"

	"github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	"github.com/initia-labs/rollytics/types"
)

// processEVMTransferEvent processes an EVM transfer event and updates the balance map.
// It extracts transfer information from the event log and updates balances for both sender and receiver.
// Returns true if the event was successfully processed, false otherwise.
func processEVMTransferEvent(logger *slog.Logger, evmLog types.EvmLog, balanceMap map[utils.BalanceChangeKey]sdkmath.Int) bool {
	if len(evmLog.Topics) != 3 || evmLog.Topics[0] != EVM_TRANSFER_TOPIC {
		return false
	}

	denom := strings.ToLower(evmLog.Address)
	fromAddr := evmLog.Topics[1]
	toAddr := evmLog.Topics[2]

	// Parse amount from hex string in evmLog.Data
	amount, ok := utils.ParseHexAmountToSDKInt(evmLog.Data)
	if !ok {
		logger.Error("failed to parse amount, skipping the entry")
		return false
	}

	logger.Warn("evm log", slog.String("denom", denom))

	// Update sender's balance (subtract)
	if fromAddr != EMPTY_ADDRESS {
		fromKey := utils.NewBalanceChangeKey(denom, fromAddr)
		if balance, ok := balanceMap[fromKey]; !ok {
			balanceMap[fromKey] = sdkmath.ZeroInt().Sub(amount)
		} else {
			balanceMap[fromKey] = balance.Sub(amount)
		}
	}

	// Update receiver's balance (add)
	if toAddr != EMPTY_ADDRESS {
		toKey := utils.NewBalanceChangeKey(denom, toAddr)
		if balance, ok := balanceMap[toKey]; !ok {
			balanceMap[toKey] = sdkmath.ZeroInt().Add(amount)
		} else {
			balanceMap[toKey] = balance.Add(amount)
		}
	}

	return true
}

// ProcessEvmBalanceChanges processes EVM transactions and calculates balance changes
// for each address. Returns a map of BalanceChangeKey to balance change amounts.
func ProcessEvmBalanceChanges(logger *slog.Logger, evmTxs []types.CollectedEvmTx) map[utils.BalanceChangeKey]sdkmath.Int {
	balanceMap := make(map[utils.BalanceChangeKey]sdkmath.Int)

	// Process each transaction
	for _, evmTx := range evmTxs {
		// Parse tx data to get timestamp and events
		var evmTxData types.EvmTx
		if err := json.Unmarshal(evmTx.Data, &evmTxData); err != nil {
			continue
		}

		for _, log := range evmTxData.Logs {
			processEVMTransferEvent(logger, log, balanceMap)
		}
	}

	return balanceMap
}
