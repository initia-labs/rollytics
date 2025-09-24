package tx

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
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

type byAccountTestCase struct {
	name       string
	route      string
	handler    func(*TxHandler) fiber.Handler
	seqInfo    types.SeqInfoName
	seqValue   int64
	useEdges   bool
	table      string
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
			useEdges:   false,
			table:      "tx",
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
			useEdges:   true,
			table:      "tx",
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
			useEdges:   true,
			table:      "evm_tx",
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
			useEdges:   false,
			table:      "evm_tx",
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

func (tc byAccountTestCase) requestPath() string {
	return strings.Replace(tc.route, ":account", tc.accountHex, 1)
}

func setupAccountExpectations(t *testing.T, mock sqlmock.Sqlmock, tc byAccountTestCase, accBytes []byte) {
	t.Helper()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}).AddRow(tc.accountID, accBytes))

	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY`).
		WithArgs(string(tc.seqInfo), 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "sequence"}).AddRow(string(tc.seqInfo), tc.seqValue))

	row := sqlmock.NewRows([]string{"hash", "height", "sequence", "signer_id", "data"}).
		AddRow([]byte(tc.hash), tc.height, tc.sequence, tc.accountID, tc.payload(tc.hash))

	if tc.useEdges {
		mock.ExpectQuery(`SELECT count\(\*\) FROM "` + tc.table + `" WHERE sequence IN \(SELECT`).
			WithArgs(tc.accountID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(`SELECT \* FROM "`+tc.table+`" WHERE sequence IN \(SELECT`).
			WithArgs(tc.accountID, sqlmock.AnyArg()).
			WillReturnRows(row)
	} else {
		mock.ExpectQuery(`SELECT count\(\*\) FROM "` + tc.table + `" WHERE account_ids &&`).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(`SELECT \* FROM "`+tc.table+`" WHERE account_ids &&`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(row)
	}

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
