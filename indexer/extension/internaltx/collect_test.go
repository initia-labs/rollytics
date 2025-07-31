package internaltx_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	internal_tx "github.com/initia-labs/rollytics/indexer/extension/internaltx"
	"github.com/initia-labs/rollytics/orm"
	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func setupTestDB(t *testing.T) (*orm.Database, sqlmock.Sqlmock) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	db, mock, err := testutil.NewMockDB(logger)
	require.NoError(t, err)

	return db, mock
}

func setupTestConfig() *config.Config {
	cfg := &config.Config{}

	cfg.SetDBConfig(&dbconfig.Config{
		BatchSize: 10,
	})
	cfg.SetChainConfig(&config.ChainConfig{
		ChainId: "test-chain",
		VmType:  types.EVM,
	})
	cfg.SetInternalTxConfig(&config.InternalTxConfig{
		Enabled:   true,
		BatchSize: 10,
	})

	return cfg
}

func getTestResponse() *internal_tx.InternalTxResult {
	callTraceRes := &internal_tx.DebugCallTraceBlockResponse{
		Result: []internal_tx.TransactionTrace{
			{
				TxHash: "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
		},
	}

	callTraceRes.Result[0].Result.Type = "CALL"
	callTraceRes.Result[0].Result.From = "0x1234567890123456789012345678901234567890"
	callTraceRes.Result[0].Result.To = "0x0987654321098765432109876543210987654321"
	callTraceRes.Result[0].Result.Value = "0x0100"
	callTraceRes.Result[0].Result.Gas = "0x5208"
	callTraceRes.Result[0].Result.GasUsed = "0x5208"
	callTraceRes.Result[0].Result.Input = "0x"
	callTraceRes.Result[0].Result.Calls = []internal_tx.InternalTransaction{
		{
			Type:    "CALL",
			From:    "0x0987654321098765432109876543210987654321",
			To:      "0x1111111111111111111111111111111111111111",
			Value:   "0x0025",
			Gas:     "0x1302",
			GasUsed: "0x1302",
			Input:   "0x",
			Output:  "0x00fedcba",
			Calls: []internal_tx.InternalTransaction{
				{
					Type:    "STATICCALL",
					From:    "0x1111111111111111111111111111111111111111",
					To:      "0x3333333333333333333333333333333333333333",
					Value:   "0x00",
					Gas:     "0x0800",
					GasUsed: "0x0600",
					Input:   "0xaabbccdd",
					Output:  "0x11223344",
				},
			},
		},
		{
			Type:    "DELEGATECALL",
			From:    "0x1234567890123456789012345678901234567890",
			To:      "0x2222222222222222222222222222222222222222",
			Value:   "0x00",
			Gas:     "0x3000",
			GasUsed: "0x2500",
			Input:   "0x",
			Output:  "0xbeefdead",
		},
	}

	return &internal_tx.InternalTxResult{
		Height:    100,
		CallTrace: callTraceRes,
	}
}

func TestIndexer_CollectInternalTxs(t *testing.T) {
	db, mock := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := setupTestConfig()
	indexer := internal_tx.New(cfg, logger, db)

	height := int64(100)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY "seq_info"\."name" LIMIT \$2`).
		WithArgs("evm_internal_tx", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).
			AddRow("evm_internal_tx", int64(0)))

	// Test transaction hash
	txHash, _ := util.HexToBytes("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	txHashHex := util.BytesToHex(txHash)
	require.Equal(t, "0x"+txHashHex, "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")

	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow(txHash, height, "{1,2}"))

	// Mock for hash dictionary lookup (happens first)
	mock.ExpectQuery(`SELECT \* FROM "evm_tx_hash_dict" WHERE hash IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "hash"}))

	// Mock for creating new hash dictionary entry (GORM uses RETURNING for auto-incrementing IDs)
	mock.ExpectQuery(`INSERT INTO "evm_tx_hash_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(1)))

	// Test addresses - these will be converted to AccAddress format internally
	// From: 0x1234567890123456789012345678901234567890
	// To: 0x0987654321098765432109876543210987654321
	fromAddr1, _ := util.AccAddressFromString("0x1234567890123456789012345678901234567890")
	toAddr1, _ := util.AccAddressFromString("0x0987654321098765432109876543210987654321")

	fromAddr1Hex := util.BytesToHex(fromAddr1)
	require.Equal(t, "0x"+fromAddr1Hex, "0x1234567890123456789012345678901234567890")
	toAddr1Hex := util.BytesToHex(toAddr1)
	require.Equal(t, "0x"+toAddr1Hex, "0x0987654321098765432109876543210987654321")

	// First internal call - processInternalCall is called
	// Step 1: GrepAddressesFromEvmInternalTx extracts From and To, then GetOrCreateAccountIds is called
	// This queries for both From and To addresses together
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	// Since no accounts exist, it will try to INSERT them
	// The accounts will be stored in bech32 format after AccAddressFromString conversion
	mock.ExpectQuery(`INSERT INTO "account_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(3)). // ID for fromAddr1
			AddRow(int64(4))) // ID for toAddr1
	// Step 2 & 3: GetOrCreateAccountIds for From and To addresses are now cached, so no DB queries

	// Second internal call (nested CALL)
	// From: 0x0987654321098765432109876543210987654321 (already cached as id=4)
	// To: 0x1111111111111111111111111111111111111111 (new)
	toAddr2, _ := util.AccAddressFromString("0x1111111111111111111111111111111111111111")
	toAddr2Hex := util.BytesToHex(toAddr2)
	require.Equal(t, "0x"+toAddr2Hex, "0x1111111111111111111111111111111111111111")
	// Step 1: GrepAddressesFromEvmInternalTx - From (id=4) is cached, To (id=5) is new
	// Only queries for the new address
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	// One address needs to be created
	mock.ExpectQuery(`INSERT INTO "account_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(5))) // ID for toAddr2
	// Step 2 & 3: From and To are now cached, no DB queries

	// Third internal call (STATICCALL)
	// From: 0x1111111111111111111111111111111111111111 (already cached as id=5)
	// To: 0x3333333333333333333333333333333333333333 (new)
	toAddr3, _ := util.AccAddressFromString("0x3333333333333333333333333333333333333333")
	toAddr3Hex := util.BytesToHex(toAddr3)
	require.Equal(t, "0x"+toAddr3Hex, "0x3333333333333333333333333333333333333333")
	// Step 1: GrepAddressesFromEvmInternalTx - From (id=5) is cached, To (id=7) is new
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	mock.ExpectQuery(`INSERT INTO "account_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(7))) // ID for toAddr3
	// Step 2 & 3: From and To are now cached, no DB queries

	// Fourth internal call (DELEGATECALL)
	// From: 0x1234567890123456789012345678901234567890 (already cached as id=3)
	// To: 0x2222222222222222222222222222222222222222 (new)
	toAddr4, _ := util.AccAddressFromString("0x2222222222222222222222222222222222222222")
	toAddr4Hex := util.BytesToHex(toAddr4)
	require.Equal(t, "0x"+toAddr4Hex, "0x2222222222222222222222222222222222222222")
	// Step 1: GrepAddressesFromEvmInternalTx - From (id=3) is cached, To (id=8) is new
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	mock.ExpectQuery(`INSERT INTO "account_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(8))) // ID for toAddr4
	// Step 2 & 3: From and To are now cached, no DB queries

	// Expected internal transactions:
	// 1. Top-level CALL: index=0, parent_index=-1
	// 2. First nested CALL: index=1, parent_index=0
	// 3. STATICCALL inside first nested: index=2, parent_index=1
	// 4. Second nested DELEGATECALL: index=3, parent_index=0

	value1, _ := util.HexToBytes("0x0100")
	value2, _ := util.HexToBytes("0x0025")
	value3, _ := util.HexToBytes("0x00")

	gas1, _ := util.HexToBytes("0x5208")
	gas2, _ := util.HexToBytes("0x1302")
	gas3, _ := util.HexToBytes("0x0800")
	gasUsed3, _ := util.HexToBytes("0x0600")
	gas4, _ := util.HexToBytes("0x3000")
	gasUsed4, _ := util.HexToBytes("0x2500")

	input1, _ := util.HexToBytes("0x")
	input3, _ := util.HexToBytes("0xaabbccdd")

	output2, _ := util.HexToBytes("0x00fedcba")
	output3, _ := util.HexToBytes("0x11223344")
	output4, _ := util.HexToBytes("0xbeefdead")

	// Verify the INSERT arguments contain correct index and parent_index values
	// GORM will insert batch data, so we check for the presence of expected values
	mock.ExpectExec(`INSERT INTO "evm_internal_tx"`).
		WithArgs(
			// First transaction (top-level CALL)
			int64(100),       // height
			int64(1),         // hash ID
			int64(0),         // index for top-level
			int64(-1),        // parent_index for top-level
			int64(1),         // sequence
			"CALL",           // type
			int64(3),         // from_id
			int64(4),         // to_id
			input1,           // input
			sqlmock.AnyArg(), // output
			value1,           // value
			gas1,             // gas
			gas1,             // gas_used
			sqlmock.AnyArg(), // account_ids (order not guaranteed)
			// Second transaction (first nested CALL)
			int64(100),       // height
			int64(1),         // hash ID
			int64(1),         // index for first nested
			int64(0),         // parent_index for first nested
			int64(2),         // sequence
			"CALL",           // type
			int64(4),         // from_id
			int64(5),         // to_id
			input1,           // input
			output2,          // output
			value2,           // value
			gas2,             // gas
			gas2,             // gas_used
			sqlmock.AnyArg(), // account_ids (order not guaranteed)
			// Third transaction (STATICCALL inside first nested)
			int64(100),       // height
			int64(1),         // hash ID
			int64(2),         // index for static call
			int64(1),         // parent_index for static call
			int64(3),         // sequence
			"STATICCALL",     // type
			int64(5),         // from_id
			int64(7),         // to_id
			input3,           // input
			output3,          // output
			value3,           // value
			gas3,             // gas
			gasUsed3,         // gas_used
			sqlmock.AnyArg(), // account_ids (order not guaranteed)
			// Fourth transaction (DELEGATECALL)
			int64(100),       // height
			int64(1),         // hash ID
			int64(3),         // index for delegate call
			int64(0),         // parent_index for delegate call
			int64(4),         // sequence
			"DELEGATECALL",   // type
			int64(3),         // from_id (DELEGATECALL uses original sender)
			int64(8),         // to_id
			input1,           // input
			output4,          // output
			value3,           // value
			gas4,             // gas
			gasUsed4,         // gas_used
			sqlmock.AnyArg(), // account_ids (order not guaranteed)
		).
		WillReturnResult(sqlmock.NewResult(1, 4))

	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET "sequence"="excluded"\."sequence"`).
		WithArgs("evm_internal_tx", int64(4)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	testRes := getTestResponse()
	err := indexer.CollectInternalTxs(db, testRes)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexer_CollectInternalTxs_MismatchedResults(t *testing.T) {
	db, mock := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := setupTestConfig()
	indexer := internal_tx.New(cfg, logger, db)

	height := int64(100)

	mock.ExpectBegin()

	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY "seq_info"\."name" LIMIT \$2`).
		WithArgs("evm_internal_tx", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).
			AddRow("evm_internal_tx", int64(0)))

	// Test transaction hash
	txHash, _ := util.HexToBytes("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")

	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow(txHash, height, "{1}"))

	mock.ExpectRollback()

	callTraceRes := &internal_tx.DebugCallTraceBlockResponse{
		Result: []internal_tx.TransactionTrace{
			{TxHash: "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			{TxHash: "0xabcdef1234567891abcdef1234567891abcdef1234567891abcdef1234567891"},
		},
	}
	callTraceRes.Result[0].Result.Type = "CALL"
	callTraceRes.Result[1].Result.Type = "CALL"

	err := indexer.CollectInternalTxs(db, &internal_tx.InternalTxResult{
		Height:    height,
		CallTrace: callTraceRes,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexer_CollectInternalTxs_EmptyInternalTxs(t *testing.T) {
	db, mock := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := setupTestConfig()
	indexer := internal_tx.New(cfg, logger, db)

	height := int64(100)

	mock.ExpectBegin()

	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY "seq_info"\."name" LIMIT \$2`).
		WithArgs("evm_internal_tx", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).
			AddRow("evm_internal_tx", int64(0)))

	// Test transaction hash
	txHash, _ := util.HexToBytes("0x112233445566778899aabbccddeeff00112233445566778899aabbccddeeff00")

	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow(txHash, height, "{1}"))

	// Mock for hash dictionary lookup (happens first)
	mock.ExpectQuery(`SELECT \* FROM "evm_tx_hash_dict" WHERE hash IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "hash"}))

	// Mock for creating new hash dictionary entry
	mock.ExpectQuery(`INSERT INTO "evm_tx_hash_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(100)))

	// Test addresses for empty test
	fromAddr, _ := util.AccAddressFromString("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	toAddr, _ := util.AccAddressFromString("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	fromAddrHex := util.BytesToHex(fromAddr)
	require.Equal(t, "0x"+fromAddrHex, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	toAddrHex := util.BytesToHex(toAddr)
	require.Equal(t, "0x"+toAddrHex, "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	// Process internal call - GrepAddressesFromEvmInternalTx and GetOrCreateAccountIds
	// Step 1: GrepAddressesFromEvmInternalTx extracts From and To, then GetOrCreateAccountIds is called
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	// Since no accounts exist, it will try to INSERT them
	mock.ExpectQuery(`INSERT INTO "account_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(10)). // ID for fromAddr
			AddRow(int64(11))) // ID for toAddr
	// Step 2 & 3: GetOrCreateAccountIds for From and To addresses are now cached, so no DB queries

	mock.ExpectExec(`INSERT INTO "evm_internal_tx"`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET "sequence"="excluded"\."sequence"`).
		WithArgs("evm_internal_tx", int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	callTraceRes := &internal_tx.DebugCallTraceBlockResponse{
		Result: []internal_tx.TransactionTrace{
			{TxHash: "0x112233445566778899aabbccddeeff00112233445566778899aabbccddeeff00"},
		},
	}
	callTraceRes.Result[0].Result.Type = "CALL"
	callTraceRes.Result[0].Result.From = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	callTraceRes.Result[0].Result.To = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	callTraceRes.Result[0].Result.Value = "0x00"
	callTraceRes.Result[0].Result.Gas = "0x5208"
	callTraceRes.Result[0].Result.GasUsed = "0x5208"
	callTraceRes.Result[0].Result.Input = "0x"

	err := indexer.CollectInternalTxs(db, &internal_tx.InternalTxResult{
		Height:    height,
		CallTrace: callTraceRes,
	})
	assert.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}
