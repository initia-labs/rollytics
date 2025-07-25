package internal_tx_test

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	internal_tx "github.com/initia-labs/rollytics/indexer/internal-tx"
	"github.com/initia-labs/rollytics/orm"
	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/types"
)

const (
	mockAddress1 = "cosmos1zg69v7yszg69v7yszg69v7yszg69v7ys4mp2q5"
	mockAddress2 = "cosmos1pxrk2seppxrk2seppxrk2seppxrk2sep0yx7a5"
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
		BatchSize: 100,
	})
	cfg.SetChainConfig(&config.ChainConfig{
		ChainId: "test-chain",
		VmType:  types.EVM,
	})

	return cfg
}

func getTestResponse() *internal_tx.InternalTxResult {
	callTraceRes := &internal_tx.CallTracerResponse{
		Result: []internal_tx.TracingCallInner{
			{
				TxHash: "0xabcdef1234567890",
				Result: internal_tx.TracingCall{
					Type:    "CALL",
					From:    "0x1234567890123456789012345678901234567890",
					To:      "0x0987654321098765432109876543210987654321",
					Value:   "0x100",
					Gas:     "0x5208",
					GasUsed: "0x5208",
					Input:   "0x",
					Output:  "0x",
					Calls: []types.EvmInternalTx{
						{
							Type:    "STATICCALL",
							From:    "0x1234567890123456789012345678901234567890",
							To:      "0x0987654321098765432109876543210987654321",
							Value:   "0x50",
							Gas:     "0x2604",
							GasUsed: "0x2604",
							Input:   "0x12345678000000000000000000000000111111111111111111111111111111111111111100000000000000000000000022222222222222222222222222222222222222220000000000000000000000003333333333333333333333333333333333333333",
							Output:  "0x87654321",
						},
						{
							Type:    "CALL",
							From:    "0x0987654321098765432109876543210987654321",
							To:      "0x1111111111111111111111111111111111111111",
							Value:   "0x25",
							Gas:     "0x1302",
							GasUsed: "0x1302",
							Input:   "0xabcdef00000000000000000000000000444444444444444444444444444444444444444400000000000000000000000055555555555555555555555555555555555555550000000000000000000000006666666666666666666666666666666666666666",
							Output:  "0x00fedcba",
						},
						{
							Type:    "DELEGATECALL",
							From:    "0x1234567890123456789012345678901234567890",
							To:      "0x2222222222222222222222222222222222222222",
							Value:   "0x0",
							Gas:     "0x3000",
							GasUsed: "0x2500",
							Input:   "0xdeadbeef000000000000000000000000777777777777777777777777777777777777777700000000000000000000000088888888888888888888888888888888888888880000000000000000000000009999999999999999999999999999999999999999",
							Output:  "0xbeefdead",
						},
					},
				},
			},
		},
	}

	prestateRes := &internal_tx.PrestateTracerResponse{
		Result: []internal_tx.PrestateTraceResult{
			{
				TxHash: "0xabcdef1234567890",
				Result: internal_tx.PrestateTracerTxState{
					Pre:  json.RawMessage(`{"0x1234": {"balance": "0x100"}}`),
					Post: json.RawMessage(`{"0x1234": {"balance": "0x50"}}`),
				},
			},
		},
	}
	return &internal_tx.InternalTxResult{
		Height:       100,
		CallTraceRes: callTraceRes,
		PrestateRes:  prestateRes,
	}
}

func TestIndexer_collectInternalTxs(t *testing.T) {
	db, mock := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := setupTestConfig()
	indexer := internal_tx.New(cfg, logger, db)

	height := int64(100)

	// Mock the transaction start
	mock.ExpectBegin()

	// Mock getting sequence info
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY "seq_info"\."name" LIMIT \$2`).
		WithArgs("evm_internal_tx", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).
			AddRow("evm_internal_tx", int64(0)))

	// Mock getting EVM transactions
	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow("0xabcdef1234567890", height, "{1,2}"))

	// Mock address lookups and creations for addresses from internal transactions
	// Each internal transaction will call GetOrCreateAccountIds for its unique addresses

	// Mock account lookups - we need to be flexible about the exact patterns
	// Allow for various numbers of parameters in the IN clause

	// Mock several SELECT queries for account lookups
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	mock.ExpectQuery(`INSERT INTO "account_dict".* ON CONFLICT DO NOTHING RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	mock.ExpectQuery(`INSERT INTO "account_dict".* ON CONFLICT DO NOTHING RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))

	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
	mock.ExpectQuery(`INSERT INTO "account_dict".* ON CONFLICT DO NOTHING RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3)))

	// Mock updating EVM transaction account_ids - flexible for any number of accounts
	mock.ExpectExec(`UPDATE "evm_tx" SET "account_ids"=\$1 WHERE hash = \$2 AND height = \$3`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock inserting all internal transactions in batch - use actual table name
	mock.ExpectExec(`INSERT INTO "evm_internal_tx"`).
		WillReturnResult(sqlmock.NewResult(1, 3))

	// Mock transaction commit
	mock.ExpectCommit()

	testRes := getTestResponse()
	err := indexer.CollectInternalTxs(db, testRes)
	require.NoError(t, err)

	// Check that all expectations were met
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexer_collectInternalTxs_MismatchedResults(t *testing.T) {
	db, mock := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := setupTestConfig()
	indexer := internal_tx.New(cfg, logger, db)

	height := int64(100)

	// Mock transaction start
	mock.ExpectBegin()

	// Mock getting sequence info
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY "seq_info"\."name" LIMIT \$2`).
		WithArgs("evm_internal_tx", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).
			AddRow("evm_internal_tx", int64(0)))

	// Mock getting EVM transactions
	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow("0xabcdef1234567890", height, "{1}"))

	// Mock rollback due to mismatch error
	mock.ExpectRollback()

	callTraceRes := &internal_tx.CallTracerResponse{
		Result: []internal_tx.TracingCallInner{
			{TxHash: "0xabcdef1234567890", Result: internal_tx.TracingCall{Type: "CALL", Calls: []types.EvmInternalTx{}}},
			{TxHash: "0xabcdef1234567891", Result: internal_tx.TracingCall{Type: "CALL", Calls: []types.EvmInternalTx{}}},
		},
	}

	prestateRes := &internal_tx.PrestateTracerResponse{
		Result: []internal_tx.PrestateTraceResult{
			{
				TxHash: "0xabcdef1234567890",
				Result: internal_tx.PrestateTracerTxState{
					Pre:  json.RawMessage(`{}`),
					Post: json.RawMessage(`{}`),
				},
			},
		},
	}

	err := indexer.CollectInternalTxs(db, &internal_tx.InternalTxResult{
		Height:       height,
		CallTraceRes: callTraceRes,
		PrestateRes:  prestateRes,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")

	// Check that all expectations were met
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexer_collectInternalTxs_EmptyInternalTxs(t *testing.T) {
	db, mock := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := setupTestConfig()
	indexer := internal_tx.New(cfg, logger, db)

	height := int64(100)

	// Mock transaction start
	mock.ExpectBegin()

	// Mock getting sequence info
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY "seq_info"\."name" LIMIT \$2`).
		WithArgs("evm_internal_tx", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).
			AddRow("evm_internal_tx", int64(0)))

	// Mock getting EVM transactions
	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow("0xabcdef1234567890", height, "{1}"))

	// Mock updating EVM transaction account_ids (even with empty internal txs, it still updates)
	mock.ExpectExec(`UPDATE "evm_tx" SET "account_ids"=\$1 WHERE hash = \$2 AND height = \$3`).
		WithArgs(sqlmock.AnyArg(), "0xabcdef1234567890", height).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock commit for successful completion
	mock.ExpectCommit()

	callTraceRes := &internal_tx.CallTracerResponse{
		Result: []internal_tx.TracingCallInner{
			{TxHash: "0xabcdef1234567890", Result: internal_tx.TracingCall{Type: "CALL", Calls: []types.EvmInternalTx{}}},
		},
	}

	prestateRes := &internal_tx.PrestateTracerResponse{
		Result: []internal_tx.PrestateTraceResult{
			{
				TxHash: "0xabcdef1234567890",
				Result: internal_tx.PrestateTracerTxState{
					Pre:  json.RawMessage(`{}`),
					Post: json.RawMessage(`{}`),
				},
			},
		},
	}

	err := indexer.CollectInternalTxs(db, &internal_tx.InternalTxResult{
		Height:       height,
		CallTraceRes: callTraceRes,
		PrestateRes:  prestateRes,
	})
	assert.NoError(t, err)

	// Check that all expectations were met
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIndexer_collectInternalTxs_InvalidHexValues(t *testing.T) {
	db, mock := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := setupTestConfig()
	indexer := internal_tx.New(cfg, logger, db)

	height := int64(100)

	// Mock transaction start
	mock.ExpectBegin()

	// Mock getting sequence info
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY "seq_info"\."name" LIMIT \$2`).
		WithArgs("evm_internal_tx", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).
			AddRow("evm_internal_tx", int64(0)))

	// Mock getting EVM transactions
	mock.ExpectQuery(`SELECT hash, height, account_ids FROM "evm_tx" WHERE height = \$1 ORDER BY sequence ASC`).
		WithArgs(height).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "height", "account_ids"}).
			AddRow("0xabcdef1234567890", height, "{1}"))

	// Mock rollback due to invalid hex error
	mock.ExpectRollback()

	callTraceRes := &internal_tx.CallTracerResponse{
		Result: []internal_tx.TracingCallInner{
			{
				TxHash: "0xabcdef1234567890",
				Result: internal_tx.TracingCall{
					Type:    "CALL",
					From:    "0x1234567890123456789012345678901234567890",
					To:      "0x0987654321098765432109876543210987654321",
					Value:   "0x100",
					Gas:     "0x5208",
					GasUsed: "0x5208",
					Input:   "0x",
					Output:  "0x",
					Calls: []types.EvmInternalTx{
						{
							Type:    "CALL",
							From:    "0x1234567890123456789012345678901234567890",
							To:      "0x0987654321098765432109876543210987654321",
							Value:   "invalid_hex",
							Gas:     "0x2604",
							GasUsed: "0x2604",
							Input:   "0x12345678",
							Output:  "0x87654321",
						},
					},
				},
			},
		},
	}

	prestateRes := &internal_tx.PrestateTracerResponse{
		Result: []internal_tx.PrestateTraceResult{
			{
				TxHash: "0xabcdef1234567890",
				Result: internal_tx.PrestateTracerTxState{
					Pre:  json.RawMessage(`{}`),
					Post: json.RawMessage(`{}`),
				},
			},
		},
	}

	err := indexer.CollectInternalTxs(db, &internal_tx.InternalTxResult{
		Height:       height,
		CallTraceRes: callTraceRes,
		PrestateRes:  prestateRes,
	})
	assert.Error(t, err)

	// Check that all expectations were met
	require.NoError(t, mock.ExpectationsWereMet())
}
