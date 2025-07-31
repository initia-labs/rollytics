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

func TestIndexer_collectInternalTxs(t *testing.T) {
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

	mock.ExpectExec(`INSERT INTO "evm_internal_tx"`).
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

func TestIndexer_collectInternalTxs_MismatchedResults(t *testing.T) {
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

func TestIndexer_collectInternalTxs_EmptyInternalTxs(t *testing.T) {
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
