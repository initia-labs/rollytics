package status

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/cache"
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

// newTestConfig creates a minimal config.Config for testing purposes.
func newTestConfig() *config.Config {
	return &config.Config{}
}

// setup creates a new StatusHandler with a mocked database and a test config.
func setup(t *testing.T) (*StatusHandler, sqlmock.Sqlmock, *config.Config) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	require.NoError(t, err)

	cfg := newTestConfig()
	cfg.SetChainConfig(&config.ChainConfig{ChainId: "test-chain"})
	cfg.SetInternalTxConfig(&config.InternalTxConfig{})

	dbWrapper := &orm.Database{DB: gormDB}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	baseHandler := common.NewBaseHandler(dbWrapper, cfg, logger)
	statusHandler := NewStatusHandler(baseHandler)

	return statusHandler, mock, cfg
}

func TestGetStatus(t *testing.T) {
	t.Run("success - internal tx disabled", func(t *testing.T) {
		h, mock, cfg := setup(t)
		defer lastEvmInternalTxHeight.Store(0)

		cfg.GetInternalTxConfig().Enabled = false
		cfg.GetChainConfig().VmType = types.EVM

		mock.ExpectBegin()
		rows := sqlmock.NewRows([]string{"height"}).AddRow(100)
		mock.ExpectQuery(`SELECT .* FROM "block"`).WillReturnRows(rows)
		mock.ExpectRollback()

		app := fiber.New()
		app.Get("/status", h.GetStatus)

		req, _ := http.NewRequest("GET", "/status", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body StatusResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		require.Equal(t, int64(100), body.Height)
		require.Equal(t, int64(0), body.InternalTxHeight)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success - internal tx enabled", func(t *testing.T) {
		h, mock, cfg := setup(t)
		defer lastEvmInternalTxHeight.Store(0)

		cfg.GetInternalTxConfig().Enabled = true
		cfg.GetChainConfig().VmType = types.EVM

		mock.ExpectBegin()
		blkRows := sqlmock.NewRows([]string{"height"}).AddRow(100)
		mock.ExpectQuery(`SELECT .* FROM "block"`).WillReturnRows(blkRows)

		intTxRows := sqlmock.NewRows([]string{"height"}).AddRow(95)
		mock.ExpectQuery(`SELECT .* FROM "evm_internal_tx"`).WillReturnRows(intTxRows)

		existsRows := sqlmock.NewRows([]string{"1"}).AddRow(1)
		mock.ExpectQuery(`SELECT 1 FROM "block"`).WillReturnRows(existsRows)
		mock.ExpectRollback()

		app := fiber.New()
		app.Get("/status", h.GetStatus)

		req, _ := http.NewRequest("GET", "/status", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body StatusResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		require.Equal(t, int64(100), body.Height)
		require.Equal(t, int64(95), body.InternalTxHeight)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success - internal tx enabled but no records", func(t *testing.T) {
		h, mock, cfg := setup(t)
		defer lastEvmInternalTxHeight.Store(0)

		cfg.GetInternalTxConfig().Enabled = true
		cfg.GetChainConfig().VmType = types.EVM

		mock.ExpectBegin()
		blkRows := sqlmock.NewRows([]string{"height"}).AddRow(100)
		mock.ExpectQuery(`SELECT .* FROM "block"`).WillReturnRows(blkRows)

		// Return gorm.ErrRecordNotFound
		mock.ExpectQuery(`SELECT .* FROM "evm_internal_tx"`).WillReturnError(gorm.ErrRecordNotFound)

		// When no internal tx, it should check for existing blocks with tx_count > 0
		// Let's assume none exist, so internalTxHeight should equal lastBlock.Height
		existsRows := sqlmock.NewRows([]string{"1"}) // No rows returned
		mock.ExpectQuery(`SELECT 1 FROM "block"`).WillReturnRows(existsRows)
		mock.ExpectRollback()

		app := fiber.New()
		app.Get("/status", h.GetStatus)

		req, _ := http.NewRequest("GET", "/status", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body StatusResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		require.Equal(t, int64(100), body.Height)
		require.Equal(t, int64(100), body.InternalTxHeight)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("failure - database error", func(t *testing.T) {
		h, mock, cfg := setup(t)
		defer lastEvmInternalTxHeight.Store(0)

		cfg.GetInternalTxConfig().Enabled = false
		cfg.GetChainConfig().VmType = types.EVM

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT .* FROM "block"`).WillReturnError(gorm.ErrInvalidDB)
		mock.ExpectRollback()

		app := fiber.New()
		app.Get("/status", h.GetStatus)

		req, _ := http.NewRequest("GET", "/status", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("cache expiration with known bug", func(t *testing.T) {
		h, mock, cfg := setup(t)
		defer lastEvmInternalTxHeight.Store(0)

		cfg.GetInternalTxConfig().Enabled = false
		cfg.GetChainConfig().VmType = types.EVM

		app := fiber.New()
		// Apply the cache middleware with sub-second expiration
		app.Get("/status", cache.WithExpiration(250*time.Millisecond), h.GetStatus)

		// 1. First request - should be a cache miss and hit the DB
		mock.ExpectBegin()
		rows1 := sqlmock.NewRows([]string{"height"}).AddRow(100)
		mock.ExpectQuery(`SELECT .* FROM "block"`).WillReturnRows(rows1)
		mock.ExpectRollback()

		req1, _ := http.NewRequest("GET", "/status", nil)
		resp1, err := app.Test(req1, -1)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp1.StatusCode)
		require.Equal(t, "miss", resp1.Header.Get("X-Cache"))
		require.NoError(t, mock.ExpectationsWereMet())

		// 2. Wait shorter than expiration - should be a cache hit
		time.Sleep(100 * time.Millisecond)

		req2, _ := http.NewRequest("GET", "/status", nil)
		resp2, err := app.Test(req2, -1)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp2.StatusCode)
		require.Equal(t, "hit", resp2.Header.Get("X-Cache"))

		// 3. Wait longer than expiration - SHOULD be a miss
		time.Sleep(time.Second)

		req3, _ := http.NewRequest("GET", "/status", nil)
		resp3, err := app.Test(req3, -1)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp3.StatusCode)

		// This is the crucial assertion that confirms the bug's behavior
		require.Equal(t, "hit", resp3.Header.Get("X-Cache"), "This confirms the bug: cache did not expire after 250ms")
		t.Log("Test confirmed the known bug: cache was still a 'hit' after expiration period.")
	})
}
