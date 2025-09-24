package tx

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/config"
	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func init() {
	util.InitializeCaches(&config.CacheConfig{
		AccountCacheSize:   1024,
		NftCacheSize:       1024,
		MsgTypeCacheSize:   256,
		TypeTagCacheSize:   256,
		EvmTxHashCacheSize: 1024,
	})
}

func newTxHandlerWithMockDB(tb testing.TB) (*TxHandler, sqlmock.Sqlmock) {
	tb.Helper()
	req := require.New(tb)

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	db, mock, err := testutil.NewMockDB(logger)
	req.NoError(err)

	cfg := &config.Config{}
	cfg.SetDBConfig(&dbconfig.Config{})
	cfg.SetChainConfig(&config.ChainConfig{ChainId: "test-chain", VmType: types.EVM})
	cfg.SetInternalTxConfig(&config.InternalTxConfig{Enabled: true})

	base := common.NewBaseHandler(db, cfg, logger)
	handler := NewTxHandler(base)

	return handler, mock
}

func TestGetTxsByAccountUsesLegacyArrayWhenBackfillIncomplete(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	accountHex := "0x1"
	accBytes, err := util.AccAddressFromString(accountHex)
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}).AddRow(int64(1), accBytes))
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY`).
		WithArgs(string(types.SeqInfoTxEdgeBackfill), 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).AddRow(string(types.SeqInfoTxEdgeBackfill), int64(5)))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx" WHERE account_ids &&`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	txRows := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte{0xAA}, int64(100), int64(10), int64(1), legacyTxPayload("0xAA"))

	mock.ExpectQuery(`SELECT \* FROM "tx" WHERE account_ids &&`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(txRows)
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/txs/by_account/:account", handler.GetTxsByAccount)

	req := httptest.NewRequest(fiber.MethodGet, "/indexer/tx/v1/txs/by_account/"+accountHex, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTxsByAccountUsesEdgeTablesWhenBackfillComplete(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	accountHex := "0x2"
	accBytes, err := util.AccAddressFromString(accountHex)
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}).AddRow(int64(2), accBytes))
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY`).
		WithArgs(string(types.SeqInfoTxEdgeBackfill), 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).AddRow(string(types.SeqInfoTxEdgeBackfill), int64(-1)))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx" WHERE sequence IN \(SELECT`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	txRows := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte{0xBB}, int64(110), int64(20), int64(2), legacyTxPayload("0xBB"))

	mock.ExpectQuery(`SELECT \* FROM "tx" WHERE sequence IN \(SELECT`).
		WithArgs(int64(2), sqlmock.AnyArg()).
		WillReturnRows(txRows)
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/txs/by_account/:account", handler.GetTxsByAccount)

	req := httptest.NewRequest(fiber.MethodGet, "/indexer/tx/v1/txs/by_account/"+accountHex, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetEvmTxsByAccountUsesEdgeTablesWhenComplete(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	accountHex := "0x3"
	accBytes, err := util.AccAddressFromString(accountHex)
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}).AddRow(int64(3), accBytes))
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY`).
		WithArgs(string(types.SeqInfoEvmTxEdgeBackfill), 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).AddRow(string(types.SeqInfoEvmTxEdgeBackfill), int64(-1)))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "evm_tx" WHERE sequence IN \(SELECT`).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	txRows := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte{0xCC}, int64(120), int64(30), int64(3), evmTxPayload("0xCC"))

	mock.ExpectQuery(`SELECT \* FROM "evm_tx" WHERE sequence IN \(SELECT`).
		WithArgs(int64(3), sqlmock.AnyArg()).
		WillReturnRows(txRows)
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/evm-txs/by_account/:account", handler.GetEvmTxsByAccount)

	req := httptest.NewRequest(fiber.MethodGet, "/indexer/tx/v1/evm-txs/by_account/"+accountHex, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetEvmTxsByAccountUsesLegacyArraysWhenIncomplete(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	accountHex := "0x4"
	accBytes, err := util.AccAddressFromString(accountHex)
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}).AddRow(int64(4), accBytes))
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY`).
		WithArgs(string(types.SeqInfoEvmTxEdgeBackfill), 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).AddRow(string(types.SeqInfoEvmTxEdgeBackfill), int64(10)))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "evm_tx" WHERE account_ids &&`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	txRows := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte{0xDD}, int64(130), int64(40), int64(4), evmTxPayload("0xDD"))

	mock.ExpectQuery(`SELECT \* FROM "evm_tx" WHERE account_ids &&`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(txRows)
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/evm-txs/by_account/:account", handler.GetEvmTxsByAccount)

	req := httptest.NewRequest(fiber.MethodGet, "/indexer/tx/v1/evm-txs/by_account/"+accountHex, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func legacyTxPayload(hash string) []byte {
	payload := map[string]any{
		"txhash":     hash,
		"height":     "100",
		"logs":       []any{},
		"gas_wanted": "0",
		"gas_used":   "0",
		"timestamp":  "2020-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(payload)
	return data
}

func evmTxPayload(hash string) []byte {
	payload := map[string]any{
		"blockHash":         "0x0",
		"blockNumber":       "0x1",
		"cumulativeGasUsed": "0x0",
		"effectiveGasPrice": "0x0",
		"from":              "0x0",
		"gasUsed":           "0x0",
		"logs":              []any{},
		"logsBloom":         "0x0",
		"status":            "0x1",
		"to":                "0x1",
		"transactionHash":   hash,
		"transactionIndex":  "0x0",
		"type":              "0x0",
	}
	data, _ := json.Marshal(payload)
	return data
}
