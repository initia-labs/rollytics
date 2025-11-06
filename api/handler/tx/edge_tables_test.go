package tx

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"github.com/initia-labs/rollytics/util/common-handler/common"
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

type byAccountTestCase struct {
	name       string
	route      string
	handler    func(*TxHandler) fiber.Handler
	seqInfo    types.SeqInfoName
	seqValue   int64
	table      string
	edgeTable  string
	payload    func(string) []byte
	hash       string
	accountHex string
	accountID  int64
	height     int64
	sequence   int64
}

func TestAccountHandlersByAccount(t *testing.T) {
	cases := []byAccountTestCase{
		{
			name:       "tx legacy path",
			route:      "/indexer/tx/v1/txs/by_account/:account",
			handler:    func(th *TxHandler) fiber.Handler { return th.GetTxsByAccount },
			seqInfo:    types.SeqInfoTxEdgeBackfill,
			seqValue:   5,
			table:      "tx",
			edgeTable:  types.CollectedTxAccount{}.TableName(),
			payload:    legacyTxPayload,
			hash:       "0xAA",
			accountHex: "0x1",
			accountID:  1,
			height:     100,
			sequence:   10,
		},
		{
			name:       "tx edge path",
			route:      "/indexer/tx/v1/txs/by_account/:account",
			handler:    func(th *TxHandler) fiber.Handler { return th.GetTxsByAccount },
			seqInfo:    types.SeqInfoTxEdgeBackfill,
			seqValue:   -1,
			table:      "tx",
			edgeTable:  types.CollectedTxAccount{}.TableName(),
			payload:    legacyTxPayload,
			hash:       "0xBB",
			accountHex: "0x2",
			accountID:  2,
			height:     110,
			sequence:   20,
		},
		{
			name:       "evm tx edge path",
			route:      "/indexer/tx/v1/evm-txs/by_account/:account",
			handler:    func(th *TxHandler) fiber.Handler { return th.GetEvmTxsByAccount },
			seqInfo:    types.SeqInfoEvmTxEdgeBackfill,
			seqValue:   -1,
			table:      "evm_tx",
			edgeTable:  types.CollectedEvmTxAccount{}.TableName(),
			payload:    evmTxPayload,
			hash:       "0xCC",
			accountHex: "0x3",
			accountID:  3,
			height:     120,
			sequence:   30,
		},
		{
			name:       "evm tx legacy path",
			route:      "/indexer/tx/v1/evm-txs/by_account/:account",
			handler:    func(th *TxHandler) fiber.Handler { return th.GetEvmTxsByAccount },
			seqInfo:    types.SeqInfoEvmTxEdgeBackfill,
			seqValue:   10,
			table:      "evm_tx",
			edgeTable:  types.CollectedEvmTxAccount{}.TableName(),
			payload:    evmTxPayload,
			hash:       "0xDD",
			accountHex: "0x4",
			accountID:  4,
			height:     130,
			sequence:   40,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, mock := newTxHandlerWithMockDB(t)
			accBytes, err := util.AccAddressFromString(tc.accountHex)
			require.NoError(t, err)

			setupAccountExpectations(t, mock, tc, accBytes)

			app := fiber.New()
			app.Get(tc.route, tc.handler(handler))

			req := httptest.NewRequest(fiber.MethodGet, tc.requestPath(), nil)
			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetTxs_EdgePathWithMsgFilter(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	const (
		route       = "/indexer/tx/v1/txs?msgs=cosmos.bank.v1beta1.MsgSend"
		msgType     = "cosmos.bank.v1beta1.MsgSend"
		msgTypeID   = int64(42)
		expectedSeq = int64(15)
		height      = int64(120)
		hash        = "0xEDGE"
	)

	row := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte(hash), height, expectedSeq, int64(0), legacyTxPayload(hash))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "msg_type_dict" WHERE msg_type IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "msg_type"}).AddRow(msgTypeID, msgType))

	// Add transaction expectations for GetCountWithTimeout
	mock.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT\("sequence"\)\) FROM "` + types.CollectedTxMsgType{}.TableName() + `" WHERE msg_type_id = ANY`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "tx" WHERE sequence IN \(SELECT DISTINCT \"sequence\" FROM \"tx_msg_types\" WHERE msg_type_id = ANY\(\$1\) ORDER BY sequence DESC LIMIT \$2\)`).
		WithArgs(sqlmock.AnyArg(), int64(common.DefaultLimit)).
		WillReturnRows(row)
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/txs", handler.GetTxs)

	req := httptest.NewRequest(fiber.MethodGet, route, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTxs_EdgePathWithMsgFilter_CustomLimit(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	const (
		limit       = 25
		msgType     = "cosmos.bank.v1beta1.MsgCustom"
		msgTypeID   = int64(43)
		expectedSeq = int64(99)
		height      = int64(321)
		hash        = "0xEDGE_CUSTOM"
	)

	route := fmt.Sprintf("/indexer/tx/v1/txs?msgs=%s&pagination.limit=%d", msgType, limit)

	row := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte(hash), height, expectedSeq, int64(0), legacyTxPayload(hash))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "msg_type_dict" WHERE msg_type IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "msg_type"}).AddRow(msgTypeID, msgType))

	// Add transaction expectations for GetCountWithTimeout
	mock.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT\("sequence"\)\) FROM "` + types.CollectedTxMsgType{}.TableName() + `" WHERE msg_type_id = ANY`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "tx" WHERE sequence IN \(SELECT DISTINCT \"sequence\" FROM \"tx_msg_types\" WHERE msg_type_id = ANY\(\$1\) ORDER BY sequence DESC LIMIT \$2\)`).
		WithArgs(sqlmock.AnyArg(), int64(limit)).
		WillReturnRows(row)
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/txs", handler.GetTxs)

	req := httptest.NewRequest(fiber.MethodGet, route, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTxsByHeight_EdgePathWithMsgFilter(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	const (
		heightParam = "123"
		route       = "/indexer/tx/v1/txs/by_height/" + heightParam + "?msgs=cosmos.gov.v1.MsgVote"
		msgType     = "cosmos.gov.v1.MsgVote"
		msgTypeID   = int64(77)
		height      = int64(123)
		sequence    = int64(55)
		hash        = "0xHEIGHT"
	)

	row := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte(hash), height, sequence, int64(0), legacyTxPayload(hash))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "msg_type_dict" WHERE msg_type IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "msg_type"}).AddRow(msgTypeID, msgType))

	// Add transaction expectations for GetCountWithTimeout
	mock.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx" WHERE height = \$1 AND sequence IN \(SELECT DISTINCT "sequence" FROM "`+types.CollectedTxMsgType{}.TableName()+`" WHERE msg_type_id = ANY\(\$2\)\)`).
		WithArgs(height, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "tx" WHERE height = \$1 AND sequence IN \(SELECT DISTINCT \"sequence\" FROM \"tx_msg_types\" WHERE msg_type_id = ANY\(\$2\)\) ORDER BY sequence DESC LIMIT \$3`).
		WithArgs(height, sqlmock.AnyArg(), int64(common.DefaultLimit)).
		WillReturnRows(row)
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/txs/by_height/:height", handler.GetTxsByHeight)

	req := httptest.NewRequest(fiber.MethodGet, route, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTxsByHeight_EdgePathWithMsgFilter_CustomLimit(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	const (
		heightParam = "456"
		limit       = 30
		msgType     = "cosmos.gov.v1.MsgTally"
		msgTypeID   = int64(88)
		height      = int64(456)
		sequence    = int64(77)
		hash        = "0xHEIGHT_LIMIT"
	)

	route := fmt.Sprintf("/indexer/tx/v1/txs/by_height/%s?msgs=%s&pagination.limit=%d", heightParam, msgType, limit)

	row := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte(hash), height, sequence, int64(0), legacyTxPayload(hash))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "msg_type_dict" WHERE msg_type IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "msg_type"}).AddRow(msgTypeID, msgType))

	// Add transaction expectations for GetCountWithTimeout
	mock.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx" WHERE height = \$1 AND sequence IN \(SELECT DISTINCT "sequence" FROM "`+types.CollectedTxMsgType{}.TableName()+`" WHERE msg_type_id = ANY\(\$2\)\)`).
		WithArgs(height, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "tx" WHERE height = \$1 AND sequence IN \(SELECT DISTINCT \"sequence\" FROM \"tx_msg_types\" WHERE msg_type_id = ANY\(\$2\)\) ORDER BY sequence DESC LIMIT \$3`).
		WithArgs(height, sqlmock.AnyArg(), int64(limit)).
		WillReturnRows(row)
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/txs/by_height/:height", handler.GetTxsByHeight)

	req := httptest.NewRequest(fiber.MethodGet, route, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTxs_LegacyPathWithMsgFilter(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	const (
		route     = "/indexer/tx/v1/txs?msgs=cosmos.bank.v1beta1.MsgLegacy"
		msgType   = "cosmos.bank.v1beta1.MsgLegacy"
		msgTypeID = int64(101)
		height    = int64(222)
		sequence  = int64(12)
		hash      = "0xLEGACY"
	)

	row := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte(hash), height, sequence, int64(0), legacyTxPayload(hash))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "msg_type_dict" WHERE msg_type IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "msg_type"}).AddRow(msgTypeID, msgType))

	// Add transaction expectations for GetCountWithTimeout
	mock.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT\("sequence"\)\) FROM "` + types.CollectedTxMsgType{}.TableName() + `" WHERE msg_type_id = ANY`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "tx" WHERE sequence IN \(SELECT DISTINCT \"sequence\" FROM \"tx_msg_types\" WHERE msg_type_id = ANY\(\$1\) ORDER BY sequence DESC LIMIT \$2\)`).
		WithArgs(sqlmock.AnyArg(), int64(common.DefaultLimit)).
		WillReturnRows(row)

	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/txs", handler.GetTxs)

	req := httptest.NewRequest(fiber.MethodGet, route, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTxs_NoFilterLegacyPath(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)

	const (
		route    = "/indexer/tx/v1/txs"
		height   = int64(333)
		sequence = int64(44)
		hash     = "0xNOFILTER"
	)

	row := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte(hash), height, sequence, int64(0), legacyTxPayload(hash))

	mock.ExpectBegin()

	// Add transaction expectations for GetCountWithTimeout
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(sequence\), 0\) FROM "` + types.CollectedTxMsgType{}.TableName() + `"`).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(sequence))

	mock.ExpectQuery(`SELECT \* FROM "tx" WHERE sequence IN \(SELECT DISTINCT \"sequence\" FROM \"tx_msg_types\" ORDER BY sequence DESC LIMIT \$1\)`).
		WithArgs(int64(common.DefaultLimit)).
		WillReturnRows(row)

	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/tx/v1/txs", handler.GetTxs)

	req := httptest.NewRequest(fiber.MethodGet, route, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func (tc byAccountTestCase) requestPath() string {
	return strings.Replace(tc.route, ":account", tc.accountHex, 1)
}

func setupAccountExpectations(t *testing.T, mock sqlmock.Sqlmock, tc byAccountTestCase, accBytes []byte) {
	t.Helper()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}).AddRow(tc.accountID, accBytes))

	row := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte(tc.hash), tc.height, tc.sequence, tc.accountID, tc.payload(tc.hash))

	// Add transaction expectations for GetCountWithTimeout
	mock.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT\("sequence"\)\) FROM "` + tc.edgeTable + `" WHERE account_id = \$1`).
		WithArgs(tc.accountID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "`+tc.table+`" WHERE sequence IN \(SELECT`).
		WithArgs(tc.accountID, sqlmock.AnyArg()).
		WillReturnRows(row)

	mock.ExpectRollback()
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

func TestBuildEdgeQueryForGetTxs(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)
	req := require.New(t)

	// Test only one case: "with msg type filters and count total"
	msgTypeIds := []int64{1, 2, 3}
	pagination := &common.Pagination{
		CountTotal: true,
		Limit:      25,
		Order:      common.OrderDesc,
	}
	expectedTotal := int64(75)

	// Setup mock expectations for GetCountWithTimeout
	mock.ExpectBegin() // GORM transaction begin
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT\("sequence"\)\) FROM "` + types.CollectedTxMsgType{}.TableName() + `" WHERE msg_type_id = ANY`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(expectedTotal))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit() // GORM transaction commit

	// Call the function
	query, total, err := buildEdgeQueryForGetTxs(handler.BaseHandler.GetDatabase().DB, msgTypeIds, pagination)

	// Verify results
	req.NoError(err)
	req.NotNil(query)
	req.Equal(expectedTotal, total)

	// Verify all expectations were met
	req.NoError(mock.ExpectationsWereMet())
}

func TestBuildEdgeQueryForGetTxs_NoFilter(t *testing.T) {
	handler, mock := newTxHandlerWithMockDB(t)
	req := require.New(t)

	// Test case: "no filter with count total"
	var msgTypeIds []int64 // empty slice = no filter
	pagination := &common.Pagination{
		CountTotal: true,
		Limit:      25,
		Order:      common.OrderDesc,
	}
	expectedTotal := int64(150)

	// Setup mock expectations for GetOptimizedCount (no filter case)
	// This should use MAX(sequence) optimization
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(sequence\), 0\) FROM "tx_msg_types"`).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(expectedTotal))

	// Call the function
	query, total, err := buildEdgeQueryForGetTxs(handler.BaseHandler.GetDatabase().DB, msgTypeIds, pagination)

	// Verify results
	req.NoError(err)
	req.NotNil(query)
	req.Equal(expectedTotal, total)

	// Verify all expectations were met
	req.NoError(mock.ExpectationsWereMet())
}
