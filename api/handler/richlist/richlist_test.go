package richlist

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/cache"
	"github.com/initia-labs/rollytics/util/common-handler/common"
)

func init() {
	cache.InitializeCaches(&config.CacheConfig{
		AccountCacheSize:          1024,
		NftCacheSize:              1024,
		MsgTypeCacheSize:          256,
		TypeTagCacheSize:          256,
		MoveDenomCacheSize:        1024,
		EvmTxHashCacheSize:        1024,
		EvmDenomContractCacheSize: 1024,
		ValidatorCacheSize:        1024,
	})
}

func newRichListHandlerWithMockDB(tb testing.TB, vmType types.VMType) (*RichListHandler, sqlmock.Sqlmock) {
	tb.Helper()
	req := require.New(tb)

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	db, mock, err := testutil.NewMockDB(logger)
	req.NoError(err)

	cfg := &config.Config{}
	cfg.SetDBConfig(&dbconfig.Config{})
	cfg.SetChainConfig(&config.ChainConfig{ChainId: "test-chain", VmType: vmType})

	base := common.NewBaseHandler(db, cfg, logger)
	return NewRichListHandler(base, cfg), mock
}

// Regression: a non-zero offset can return an empty page even when move/<addr> rows exist.
// The handler must keep the original denom and not fall back to the FA symbol.
func TestGetTokenHolders_MoveDenomKeepsOriginalOnEmptyOffsetPage(t *testing.T) {
	h, mock := newRichListHandlerWithMockDB(t, types.MoveVM)

	moveDenom := "move/4e11c0a219f362e4d0e1f131699aa83bee40ebc8701b424373a8517d0c9e85fb"
	symbolDenom := "cbl iusd-slp"
	metadataAddr := "0x4e11c0a219f362e4d0e1f131699aa83bee40ebc8701b424373a8517d0c9e85fb"

	// If the bug returns, resolveMoveSymbolDenom would hit this cache and query symbolDenom.
	cache.SetMoveDenomCache(metadataAddr, symbolDenom)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT count\(\*\) FROM "rich_list" WHERE denom = \$1`).
		WithArgs(moveDenom).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT .* FROM "rich_list" WHERE denom = \$1 ORDER BY amount DESC LIMIT \$2 OFFSET \$3`).
		WithArgs(moveDenom, 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "denom", "amount"}))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "rich_list" WHERE denom = \$1`).
		WithArgs(moveDenom).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/richlist/v1/:denom", h.GetTokenHolders)

	req := httptest.NewRequestWithContext(t.Context(),
		fiber.MethodGet,
		"/indexer/richlist/v1/move%2F4e11c0a219f362e4d0e1f131699aa83bee40ebc8701b424373a8517d0c9e85fb?pagination.limit=1&pagination.offset=1",
		nil,
	)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var body TokenHoldersResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Empty(t, body.Holders)
	require.Equal(t, "1", body.Pagination.Total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTokenHolders_MoveDenomFallsBackToSymbolWhenMissing(t *testing.T) {
	h, mock := newRichListHandlerWithMockDB(t, types.MoveVM)

	moveDenom := "move/4e11c0a219f362e4d0e1f131699aa83bee40ebc8701b424373a8517d0c9e85fb"
	symbolDenom := "cbl iusd-slp"
	metadataAddr := "0x4e11c0a219f362e4d0e1f131699aa83bee40ebc8701b424373a8517d0c9e85fb"
	cache.SetMoveDenomCache(metadataAddr, symbolDenom)

	accountID := int64(176)
	amount := "1087897"

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT count\(\*\) FROM "rich_list" WHERE denom = \$1`).
		WithArgs(moveDenom).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT .* FROM "rich_list" WHERE denom = \$1 ORDER BY amount DESC LIMIT \$2`).
		WithArgs(symbolDenom, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "denom", "amount"}).
			AddRow(accountID, symbolDenom, amount))
	mock.ExpectQuery(`SELECT id, account FROM "account_dict" WHERE id IN \((\$1|176)\)`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account"}).
			AddRow(accountID, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14}))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "rich_list" WHERE denom = \$1`).
		WithArgs(symbolDenom).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	app := fiber.New()
	app.Get("/indexer/richlist/v1/:denom", h.GetTokenHolders)

	req := httptest.NewRequestWithContext(t.Context(),
		fiber.MethodGet,
		"/indexer/richlist/v1/move%2F4e11c0a219f362e4d0e1f131699aa83bee40ebc8701b424373a8517d0c9e85fb",
		nil,
	)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var body TokenHoldersResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Holders, 1)
	require.Equal(t, amount, body.Holders[0].Amount)
	require.Equal(t, "1", body.Pagination.Total)
	require.NoError(t, mock.ExpectationsWereMet())
}
