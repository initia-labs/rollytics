package nft

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/util/cache"
	"github.com/initia-labs/rollytics/util/common-handler/common"
)

func TestCollectionAccountCountUsesTimeoutAndPreservesPage(t *testing.T) {
	cache.InitializeCaches(&config.CacheConfig{
		AccountCacheSize: 32, NftCacheSize: 32, MsgTypeCacheSize: 32, TypeTagCacheSize: 32,
		MoveDenomCacheSize: 32, EvmTxHashCacheSize: 32, EvmDenomContractCacheSize: 32, ValidatorCacheSize: 32,
	})
	for _, tc := range []struct {
		name, query, total string
		timeout            bool
	}{
		{name: "exact total", total: "0"},
		{name: "timeout keeps page available", total: "-1", timeout: true},
		{name: "count disabled", query: "?pagination.count_total=false", total: "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			db, mock, err := testutil.NewMockDB(logger)
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			account := make([]byte, 20)
			_, err = rand.Read(account)
			require.NoError(t, err)
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE account IN`).WithArgs(account).
				WillReturnRows(sqlmock.NewRows([]string{"id", "account"}).AddRow(7, account))
			if tc.query == "" {
				mock.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
				count := mock.ExpectQuery(`SELECT COUNT\(DISTINCT\("nft_collection"\."addr"\)\) FROM "nft_collection" INNER JOIN nft ON nft_collection.addr = nft.collection_addr WHERE nft.owner_id = \$1`).WithArgs(7)
				if tc.timeout {
					count.WillReturnError(errors.New("canceling statement due to statement timeout"))
					mock.ExpectExec(`ROLLBACK TO SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
				} else {
					count.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
					mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
				}
			}
			// The count's DISTINCT column must not replace the page's full projection.
			mock.ExpectQuery(`SELECT DISTINCT "nft_collection"\."addr","nft_collection"\."height",.* FROM "nft_collection" INNER JOIN nft ON nft_collection.addr = nft.collection_addr WHERE nft.owner_id = \$1 ORDER BY nft_collection.height DESC LIMIT \$2`).
				WithArgs(7, common.DefaultLimit).WillReturnRows(sqlmock.NewRows([]string{"addr", "height"}))
			mock.ExpectQuery(`SELECT \* FROM "account_dict" WHERE id IN \(NULL\)`).
				WillReturnRows(sqlmock.NewRows([]string{"id", "account"}))
			mock.ExpectRollback()
			handler := NewNftHandler(common.NewBaseHandler(db, &config.Config{}, logger))
			app := fiber.New()
			app.Get("/collections/by_account/:account", handler.GetCollectionsByAccount)
			req := httptest.NewRequestWithContext(t.Context(), fiber.MethodGet, "/collections/by_account/0x"+hex.EncodeToString(account)+tc.query, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			var body CollectionsResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			require.Equal(t, tc.total, body.Pagination.Total)
			require.Empty(t, body.Collections)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
