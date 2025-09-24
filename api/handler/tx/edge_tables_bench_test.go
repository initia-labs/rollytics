package tx

import (
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"log/slog"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func BenchmarkGetTxsByAccount(b *testing.B) {
	const txCount = 20000

	cases := []struct {
		name     string
		useEdges bool
	}{
		{name: "legacy_array", useEdges: false},
		{name: "edge_tables", useEdges: true},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			if !tc.useEdges {
				b.Skip("legacy array benchmark requires PostgreSQL array operators; skipping in SQLite-backed benchmark")
			}

			handler, cleanup, accountHex := setupSQLiteBenchmarkHandler(b, txCount, tc.useEdges)
			defer cleanup()
			runTxsByAccountBenchmark(b, handler, accountHex)
		})
	}
}

func runTxsByAccountBenchmark(b *testing.B, handler *TxHandler, accountHex string) {
	app := fiber.New()
	app.Get("/indexer/tx/v1/txs/by_account/:account", handler.GetTxsByAccount)

	req := httptest.NewRequest(
		fiber.MethodGet,
		"/indexer/tx/v1/txs/by_account/"+accountHex+"?pagination.limit=1000",
		nil,
	)

	warmResp, err := app.Test(req, -1)
	require.NoError(b, err)
	require.Equal(b, fiber.StatusOK, warmResp.StatusCode)
	_, _ = io.Copy(io.Discard, warmResp.Body)
	_ = warmResp.Body.Close()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req, -1)
		if err != nil {
			b.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != fiber.StatusOK {
			b.Fatalf("unexpected status: %d", resp.StatusCode)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func setupSQLiteBenchmarkHandler(tb testing.TB, txCount int, useEdges bool) (*TxHandler, func(), string) {
	tb.Helper()

	require := require.New(tb)

	dsn := fmt.Sprintf("file:edge_bench_%d?mode=memory&cache=shared", time.Now().UnixNano())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(err)

	sqlDB, err := gdb.DB()
	require.NoError(err)

	cleanup := func() {
		_ = sqlDB.Close()
	}

	createBenchmarkSchema(tb, gdb)

	const accountCount = 200
	accounts := make([]struct {
		hex  string
		id   int64
		addr []byte
	}, accountCount)

	for i := 0; i < accountCount; i++ {
		hex := fmt.Sprintf("0xfeed%03x", i)
		accBytes, err := util.AccAddressFromString(hex)
		require.NoError(err)
		id := int64(i + 1)
		require.NoError(gdb.Exec(`INSERT INTO account_dict (id, account) VALUES (?, ?)`, id, accBytes).Error)
		accounts[i] = struct {
			hex  string
			id   int64
			addr []byte
		}{hex: hex, id: id, addr: accBytes}
	}

	primaryAccount := accounts[0]

	seqValue := int64(txCount)
	if useEdges {
		seqValue = -1
	}
	require.NoError(gdb.Exec(`INSERT INTO seq_info (name, sequence) VALUES (?, ?)`, string(types.SeqInfoTxEdgeBackfill), seqValue).Error)

	for i := 0; i < txCount; i++ {
		seq := int64(i + 1)
		payload := legacyTxPayload(fmt.Sprintf("0x%x", seq))

		account := accounts[i%accountCount]
		accountIDs := fmt.Sprintf("{%d}", account.id)

		require.NoError(gdb.Exec(
			`INSERT INTO tx (hash, height, sequence, signer_id, data, account_ids) VALUES (?, ?, ?, ?, ?, ?)`,
			[]byte{byte(i % 256)}, seq, seq, account.id, payload, accountIDs,
		).Error)

		if useEdges {
			require.NoError(gdb.Exec(
				`INSERT INTO tx_accounts (account_id, sequence, signer) VALUES (?, ?, ?)`,
				account.id, seq, account.id == primaryAccount.id,
			).Error)
		}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := &config.Config{}
	cfg.SetDBConfig(&dbconfig.Config{})
	cfg.SetChainConfig(&config.ChainConfig{ChainId: "test-chain", VmType: types.EVM})
	cfg.SetInternalTxConfig(&config.InternalTxConfig{Enabled: true})

	base := common.NewBaseHandler(&orm.Database{DB: gdb}, cfg, logger)
	handler := NewTxHandler(base)

	return handler, cleanup, primaryAccount.hex
}

func createBenchmarkSchema(tb testing.TB, db *gorm.DB) {
	tb.Helper()
	require := require.New(tb)

	statements := []string{
		`CREATE TABLE seq_info (
            name TEXT PRIMARY KEY,
            sequence INTEGER
        )`,
		`CREATE TABLE tx (
            hash BLOB,
            height INTEGER,
            sequence INTEGER,
            signer_id INTEGER,
            data BLOB,
            account_ids TEXT,
            msg_type_ids BLOB,
            type_tag_ids BLOB,
            nft_ids BLOB
        )`,
		`CREATE TABLE tx_accounts (
            account_id INTEGER,
            sequence INTEGER,
            signer BOOLEAN
        )`,
		`CREATE TABLE account_dict (
            id INTEGER PRIMARY KEY,
            account BLOB
        )`,
		`CREATE TABLE msg_type_dict (
            id INTEGER,
            msg_type TEXT
        )`,
		`CREATE INDEX idx_tx_sequence ON tx(sequence)`,
		`CREATE INDEX idx_tx_height ON tx(height)`,
		`CREATE INDEX idx_tx_signer ON tx(signer_id)`,
		`CREATE INDEX idx_tx_account_ids ON tx(account_ids)`,
		`CREATE INDEX idx_tx_accounts_account_sequence ON tx_accounts(account_id, sequence)`,
		`CREATE INDEX idx_tx_accounts_sequence ON tx_accounts(sequence)`,
	}

	for _, stmt := range statements {
		require.NoError(db.Exec(stmt).Error)
	}
}
