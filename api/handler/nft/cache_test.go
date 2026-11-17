package nft

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/util/common-handler/common"
)

func TestCollectionCacheThrottlesRefreshAttempts(t *testing.T) {
	t.Setenv("VM_TYPE", "evm")
	t.Setenv("CHAIN_ID", "cache-test")
	t.Setenv("RPC_URL", "http://localhost:26657")
	t.Setenv("REST_URL", "http://localhost:1317")
	t.Setenv("JSON_RPC_URL", "http://localhost:8545")
	t.Setenv("DB_DSN", "postgres://localhost/cache_test")
	t.Setenv("POLLING_INTERVAL", "1h")
	cfg, err := config.GetConfig()
	require.NoError(t, err)

	for _, tt := range []struct {
		name string
		cfg  *config.Config
		err  error
	}{
		{"empty", cfg, nil},
		{"query error", cfg, errors.New("database unavailable")},
		{"zero polling interval", &config.Config{}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			oldCols, oldHeight := collectionCacheByName, lastUpdatedHeight
			oldTime, oldUpdating := lastUpdatedTime.Load(), updating.Load()
			collectionCacheByName, lastUpdatedHeight = nil, 0
			lastUpdatedTime.Store(0)
			updating.Store(false)
			t.Cleanup(func() {
				collectionCacheByName, lastUpdatedHeight = oldCols, oldHeight
				lastUpdatedTime.Store(oldTime)
				updating.Store(oldUpdating)
			})

			db, mock, err := testutil.NewMockDB(slog.Default())
			require.NoError(t, err)
			sqlDB, err := db.DB.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })

			mock.ExpectBegin()
			query := mock.ExpectQuery(`SELECT .* FROM "nft_collection" WHERE height > \$1 ORDER BY height ASC`).WithArgs(int64(0))
			if tt.err != nil {
				query.WillReturnError(tt.err)
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"addr", "height", "name"}))
			}
			mock.ExpectRollback()

			pagination := &common.Pagination{Limit: 10, Order: common.OrderAsc}
			cols, total := getCollectionByNameFromCache(db.DB, tt.cfg, "test", pagination)
			require.Empty(t, cols)
			require.Zero(t, total)
			attemptTime := lastUpdatedTime.Load()
			require.Positive(t, attemptTime)
			getCollectionByNameFromCache(db.DB, tt.cfg, "other", pagination)
			require.Equal(t, attemptTime, lastUpdatedTime.Load(), "another search must not refresh before the interval")

			// After the interval, new collections must become searchable even after an empty or failed refresh.
			lastUpdatedTime.Store(time.Now().Add(-2 * time.Hour).UnixNano())
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT .* FROM "nft_collection" WHERE height > \$1 ORDER BY height ASC`).
				WithArgs(int64(0)).
				WillReturnRows(sqlmock.NewRows([]string{"addr", "height", "name"}).AddRow([]byte{1}, 12, "Test Collection"))
			mock.ExpectRollback()
			cols, total = getCollectionByNameFromCache(db.DB, tt.cfg, "test", pagination)
			require.EqualValues(t, 1, total)
			require.Len(t, cols, 1)
			require.Equal(t, "Test Collection", cols[0].Name)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
