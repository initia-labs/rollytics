package internaltx_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	internal_tx "github.com/initia-labs/rollytics/indexer/extension/internaltx"
	"github.com/initia-labs/rollytics/orm"
	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/types"
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
				TxHash: "0xabcdef1234567890",
			},
		},
	}

	callTraceRes.Result[0].Result.Type = "CALL"
	callTraceRes.Result[0].Result.From = "0x1234567890123456789012345678901234567890"
	callTraceRes.Result[0].Result.To = "0x0987654321098765432109876543210987654321"
	callTraceRes.Result[0].Result.Value = "0x100"
	callTraceRes.Result[0].Result.Gas = "0x5208"
	callTraceRes.Result[0].Result.GasUsed = "0x5208"
	callTraceRes.Result[0].Result.Input = "0x"
	callTraceRes.Result[0].Result.Calls = []internal_tx.InternalTransaction{
		{
			Type:    "CALL",
			From:    "0x0987654321098765432109876543210987654321",
			To:      "0x1111111111111111111111111111111111111111",
			Value:   "0x25",
			Gas:     "0x1302",
			GasUsed: "0x1302",
			Input:   "0x",
			Output:  "0x00fedcba",
			Calls: []internal_tx.InternalTransaction{
				{
					Type:    "STATICCALL",
					From:    "0x1111111111111111111111111111111111111111",
					To:      "0x3333333333333333333333333333333333333333",
					Value:   "0x0",
					Gas:     "0x800",
					GasUsed: "0x600",
					Input:   "0xaabbccdd",
					Output:  "0x11223344",
				},
			},
		},
		{
			Type:    "DELEGATECALL",
			From:    "0x1234567890123456789012345678901234567890",
			To:      "0x2222222222222222222222222222222222222222",
			Value:   "0x0",
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

	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow("0xabcdef1234567890", height, "{1,2}"))

	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	mock.ExpectQuery(`INSERT INTO "account_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(3)).
			AddRow(int64(4)))

	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	mock.ExpectQuery(`INSERT INTO "account_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(5)))

	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	mock.ExpectQuery(`INSERT INTO "account_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(7)))

	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	mock.ExpectQuery(`INSERT INTO "account_dict".*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(8)))

	// Expected internal transactions:
	// 1. Top-level CALL: index=0, parent_index=-1
	// 2. First nested CALL: index=1, parent_index=0
	// 3. STATICCALL inside first nested: index=2, parent_index=1
	// 4. Second nested DELEGATECALL: index=3, parent_index=0

	// Verify the INSERT arguments contain correct index and parent_index values
	// GORM will insert batch data, so we check for the presence of expected values
	mock.ExpectExec(`INSERT INTO "evm_internal_tx"`).
		WithArgs(
			// First transaction (top-level CALL)
			int64(100),           // height
			"0xabcdef1234567890", // hash
			int64(1),             // sequence
			int64(-1),            // parent_index for top-level
			int64(0),             // index for top-level
			"CALL",               // type
			"0x1234567890123456789012345678901234567890", // from
			"0x0987654321098765432109876543210987654321", // to
			"0x",                    // input
			sqlmock.AnyArg(),        // output
			"0x100",                 // value
			"0x5208",                // gas
			"0x5208",                // gas_used
			pq.Array([]int64{3, 4}), // account_ids
			// Second transaction (first nested CALL)
			int64(100),           // height
			"0xabcdef1234567890", // hash
			int64(2),             // sequence
			int64(0),             // parent_index for first nested
			int64(1),             // index for first nested
			"CALL",               // type
			"0x0987654321098765432109876543210987654321", // from
			"0x1111111111111111111111111111111111111111", // to
			"0x",                    // input
			"0x00fedcba",            // output
			"0x25",                  // value
			"0x1302",                // gas
			"0x1302",                // gas_used
			pq.Array([]int64{4, 5}), // account_ids
			// Third transaction (STATICCALL inside first nested)
			int64(100),           // height
			"0xabcdef1234567890", // hash
			int64(3),             // sequence
			int64(1),             // parent_index for static call
			int64(2),             // index for static call
			"STATICCALL",         // type
			"0x1111111111111111111111111111111111111111", // from
			"0x3333333333333333333333333333333333333333", // to
			"0xaabbccdd",            // input
			"0x11223344",            // output
			"0x0",                   // value
			"0x800",                 // gas
			"0x600",                 // gas_used
			pq.Array([]int64{5, 7}), // account_ids
			// Fourth transaction (DELEGATECALL)
			int64(100),           // height
			"0xabcdef1234567890", // hash
			int64(4),             // sequence
			int64(0),             // parent_index for delegate call
			int64(3),             // index for delegate call
			"DELEGATECALL",       // type
			"0x1234567890123456789012345678901234567890", // from
			"0x2222222222222222222222222222222222222222", // to
			"0x",                    // input
			"0xbeefdead",            // output
			"0x0",                   // value
			"0x3000",                // gas
			"0x2500",                // gas_used
			pq.Array([]int64{3, 8}), // account_ids
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

	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow("0xabcdef1234567890", height, "{1}"))

	mock.ExpectRollback()

	callTraceRes := &internal_tx.DebugCallTraceBlockResponse{
		Result: []internal_tx.TransactionTrace{
			{TxHash: "0xabcdef1234567890"},
			{TxHash: "0xabcdef1234567891"},
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

	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow("0xabcdef1234567890", height, "{1}"))

	mock.ExpectExec(`INSERT INTO "evm_internal_tx"`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET "sequence"="excluded"\."sequence"`).
		WithArgs("evm_internal_tx", int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	callTraceRes := &internal_tx.DebugCallTraceBlockResponse{
		Result: []internal_tx.TransactionTrace{
			{TxHash: "0xabcdef1234567890"},
		},
	}
	callTraceRes.Result[0].Result.Type = "CALL"

	err := indexer.CollectInternalTxs(db, &internal_tx.InternalTxResult{
		Height:    height,
		CallTrace: callTraceRes,
	})
	assert.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}
