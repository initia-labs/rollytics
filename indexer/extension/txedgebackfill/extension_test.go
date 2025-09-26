package txedgebackfill

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/types"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newSQLiteDatabase(t *testing.T) *orm.Database {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)

	return &orm.Database{DB: gdb}
}

func mustExec(t *testing.T, db *orm.Database, query string, args ...any) {
	t.Helper()
	require.NoError(t, db.Exec(query, args...).Error)
}

func newMockExtension(t *testing.T) (*Extension, sqlmock.Sqlmock) {
	t.Helper()

	logger := newTestLogger()

	db, mock, err := testutil.NewMockDB(logger)
	require.NoError(t, err)

	ext := &Extension{
		logger:             logger,
		db:                 db,
		batchSize:          defaultBatchSize,
		idleDelay:          defaultIdleDuration,
		hasTxTables:        true,
		hasEvmTables:       true,
		hasEvmInternalData: true,
	}

	return ext, mock
}

func expectSeqInfoLookup(mock sqlmock.Sqlmock, name types.SeqInfoName, sequence int64) {
	rows := sqlmock.NewRows([]string{"name", "sequence"}).
		AddRow(string(name), sequence)
	mock.ExpectQuery(`SELECT \* FROM "seq_info" WHERE name = \$1 ORDER BY "seq_info"\."name" LIMIT \$2`).
		WithArgs(string(name), 1).
		WillReturnRows(rows)
}

func TestBackfillTxAlreadyComplete(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoTxEdgeBackfill, -1)

	didWork, complete, err := ext.backfillTx(context.Background())
	require.NoError(t, err)
	require.False(t, didWork)
	require.True(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillTxNoPendingMarksComplete(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoTxEdgeBackfill, 10)

	mock.ExpectBegin()
	statsRows := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
		"msg_types_inserted",
		"type_tags_inserted",
		"nfts_inserted",
	}).AddRow(int64(0), nil, int64(0), int64(0), int64(0), int64(0))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(10), defaultBatchSize).
		WillReturnRows(statsRows)

	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoTxEdgeBackfill), int64(-1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	didWork, complete, err := ext.backfillTx(context.Background())
	require.NoError(t, err)
	require.False(t, didWork)
	require.True(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillTxPendingAdvancesSequence(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoTxEdgeBackfill, 25)

	mock.ExpectBegin()
	statsRows := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
		"msg_types_inserted",
		"type_tags_inserted",
		"nfts_inserted",
	}).AddRow(int64(3), int64(42), int64(3), int64(2), int64(1), int64(0))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(25), defaultBatchSize).
		WillReturnRows(statsRows)

	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoTxEdgeBackfill), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	didWork, complete, err := ext.backfillTx(context.Background())
	require.NoError(t, err)
	require.True(t, didWork)
	require.False(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillTxPendingWithoutMaxSequenceReturnsError(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoTxEdgeBackfill, 25)

	mock.ExpectBegin()
	statsRows := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
		"msg_types_inserted",
		"type_tags_inserted",
		"nfts_inserted",
	}).AddRow(int64(1), nil, int64(0), int64(0), int64(0), int64(0))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(25), defaultBatchSize).
		WillReturnRows(statsRows)

	mock.ExpectRollback()

	didWork, complete, err := ext.backfillTx(context.Background())
	require.Error(t, err)
	require.False(t, didWork)
	require.False(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillEvmTxAlreadyComplete(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoEvmTxEdgeBackfill, -1)

	didWork, complete, err := ext.backfillEvmTx(context.Background())
	require.NoError(t, err)
	require.False(t, didWork)
	require.True(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillEvmTxPendingAdvancesSequence(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoEvmTxEdgeBackfill, 7)

	mock.ExpectBegin()
	statsRows := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(5), int64(14), int64(4))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(7), defaultBatchSize).
		WillReturnRows(statsRows)

	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoEvmTxEdgeBackfill), int64(14)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	didWork, complete, err := ext.backfillEvmTx(context.Background())
	require.NoError(t, err)
	require.True(t, didWork)
	require.False(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillEvmTxNoPendingMarksComplete(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoEvmTxEdgeBackfill, 7)

	mock.ExpectBegin()
	statsRows := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(0), nil, int64(0))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(7), defaultBatchSize).
		WillReturnRows(statsRows)

	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoEvmTxEdgeBackfill), int64(-1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	didWork, complete, err := ext.backfillEvmTx(context.Background())
	require.NoError(t, err)
	require.False(t, didWork)
	require.True(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillEvmTxPendingWithoutMaxSequenceReturnsError(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoEvmTxEdgeBackfill, 7)

	mock.ExpectBegin()
	statsRows := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(2), nil, int64(0))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(7), defaultBatchSize).
		WillReturnRows(statsRows)

	mock.ExpectRollback()

	didWork, complete, err := ext.backfillEvmTx(context.Background())
	require.Error(t, err)
	require.False(t, didWork)
	require.False(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillEvmInternalTxNoPending(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoEvmInternalTxEdgeBackfill, 3)

	mock.ExpectBegin()
	statsRows := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(0), nil, int64(0))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(3), defaultBatchSize).
		WillReturnRows(statsRows)

	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoEvmInternalTxEdgeBackfill), int64(-1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	didWork, complete, err := ext.backfillEvmInternalTx(context.Background())
	require.NoError(t, err)
	require.False(t, didWork)
	require.True(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillEvmInternalTxPendingAdvancesSequence(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoEvmInternalTxEdgeBackfill, 3)

	mock.ExpectBegin()
	statsRows := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(2), int64(12), int64(2))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(3), defaultBatchSize).
		WillReturnRows(statsRows)

	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoEvmInternalTxEdgeBackfill), int64(12)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	didWork, complete, err := ext.backfillEvmInternalTx(context.Background())
	require.NoError(t, err)
	require.True(t, didWork)
	require.False(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillEvmInternalTxPendingWithoutMaxSequence(t *testing.T) {
	ext, mock := newMockExtension(t)

	expectSeqInfoLookup(mock, types.SeqInfoEvmInternalTxEdgeBackfill, 8)

	mock.ExpectBegin()
	statsRows := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(2), nil, int64(2))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(8), defaultBatchSize).
		WillReturnRows(statsRows)

	mock.ExpectRollback()

	didWork, complete, err := ext.backfillEvmInternalTx(context.Background())
	require.Error(t, err)
	require.False(t, didWork)
	require.False(t, complete)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewReturnsNilWhenNoTables(t *testing.T) {
	db := newSQLiteDatabase(t)

	ext := New(&config.Config{}, newTestLogger(), db)
	require.Nil(t, ext)
}

func TestNewReturnsExtensionWhenTxBackfillNeeded(t *testing.T) {
	db := newSQLiteDatabase(t)

	// create required tables for tx edges
	mustExec(t, db, `CREATE TABLE tx (
		sequence INTEGER,
		signer_id INTEGER,
		account_ids BLOB,
		msg_type_ids BLOB,
		type_tag_ids BLOB,
		nft_ids BLOB
	)`)
	mustExec(t, db, `CREATE TABLE tx_accounts (
		account_id INTEGER,
		sequence INTEGER,
		signer BOOLEAN
	)`)
	mustExec(t, db, `CREATE TABLE seq_info (
		name TEXT PRIMARY KEY,
		sequence INTEGER
	)`)

	inst := New(&config.Config{}, newTestLogger(), db)
	require.NotNil(t, inst)
	actual, ok := inst.(*Extension)
	require.True(t, ok)
	require.True(t, actual.hasTxTables)
	require.False(t, actual.hasEvmTables)
	require.False(t, actual.hasEvmInternalData)
	require.Equal(t, defaultBatchSize, actual.batchSize)
	require.Equal(t, defaultIdleDuration, actual.idleDelay)
}

func TestNewReturnsNilAfterBackfillComplete(t *testing.T) {
	db := newSQLiteDatabase(t)

	mustExec(t, db, `CREATE TABLE tx (
		sequence INTEGER,
		signer_id INTEGER,
		account_ids BLOB,
		msg_type_ids BLOB,
		type_tag_ids BLOB,
		nft_ids BLOB
	)`)
	mustExec(t, db, `CREATE TABLE tx_accounts (
		account_id INTEGER,
		sequence INTEGER,
		signer BOOLEAN
	)`)
	mustExec(t, db, `CREATE TABLE seq_info (
		name TEXT PRIMARY KEY,
		sequence INTEGER
	)`)
	mustExec(t, db, `INSERT INTO seq_info (name, sequence) VALUES (?, ?)`, string(types.SeqInfoTxEdgeBackfill), -1)

	inst := New(&config.Config{}, newTestLogger(), db)
	require.Nil(t, inst)
}

func TestIsBackfillReady(t *testing.T) {
	db := newSQLiteDatabase(t)

	mustExec(t, db, `CREATE TABLE seq_info (
		name TEXT PRIMARY KEY,
		sequence INTEGER
	)`)

	ready, err := isBackfillReady(db.DB, types.SeqInfoTxEdgeBackfill)
	require.NoError(t, err)
	require.False(t, ready)

	mustExec(t, db, `INSERT INTO seq_info (name, sequence) VALUES (?, ?)`, string(types.SeqInfoTxEdgeBackfill), -1)

	ready, err = isBackfillReady(db.DB, types.SeqInfoTxEdgeBackfill)
	require.NoError(t, err)
	require.True(t, ready)
}

func TestRunCompletesWhenBackfillAlreadyDone(t *testing.T) {
	logger := newTestLogger()
	db, mock, err := testutil.NewMockDB(logger)
	require.NoError(t, err)

	ext := &Extension{
		logger:       logger,
		db:           db,
		batchSize:    defaultBatchSize,
		idleDelay:    time.Millisecond,
		hasTxTables:  true,
		hasEvmTables: false,
		cfg:          &config.Config{},
	}

	expectSeqInfoLookup(mock, types.SeqInfoTxEdgeBackfill, -1)

	require.NoError(t, ext.Run(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunStopsOnContextCanceledError(t *testing.T) {
	logger := newTestLogger()
	db, mock, err := testutil.NewMockDB(logger)
	require.NoError(t, err)

	ext := &Extension{
		logger:       logger,
		db:           db,
		batchSize:    defaultBatchSize,
		idleDelay:    time.Millisecond,
		hasTxTables:  true,
		hasEvmTables: false,
		cfg:          &config.Config{},
	}

	expectSeqInfoLookup(mock, types.SeqInfoTxEdgeBackfill, 0)
	mock.ExpectBegin()
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(0), defaultBatchSize).
		WillReturnError(context.Canceled)
	mock.ExpectRollback()

	require.NoError(t, ext.Run(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunPropagatesEvmBackfillError(t *testing.T) {
	logger := newTestLogger()
	db, mock, err := testutil.NewMockDB(logger)
	require.NoError(t, err)

	ext := &Extension{
		logger:       logger,
		db:           db,
		batchSize:    defaultBatchSize,
		idleDelay:    time.Millisecond,
		hasEvmTables: true,
		cfg:          &config.Config{},
	}

	boom := fmt.Errorf("evm failure")
	expectSeqInfoLookup(mock, types.SeqInfoEvmTxEdgeBackfill, 0)
	mock.ExpectBegin()
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(0), defaultBatchSize).
		WillReturnError(boom)
	mock.ExpectRollback()

	err = ext.Run(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, boom)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunProcessesEvmBackfillToCompletion(t *testing.T) {
	logger := newTestLogger()
	db, mock, err := testutil.NewMockDB(logger)
	require.NoError(t, err)

	ext := &Extension{
		logger:             logger,
		db:                 db,
		batchSize:          defaultBatchSize,
		idleDelay:          time.Millisecond,
		hasEvmTables:       true,
		hasTxTables:        false,
		hasEvmInternalData: false,
		cfg:                &config.Config{},
	}

	// First iteration: pending rows exist, advance sequence
	expectSeqInfoLookup(mock, types.SeqInfoEvmTxEdgeBackfill, 0)
	mock.ExpectBegin()
	firstStats := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(2), int64(42), int64(2))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(0), defaultBatchSize).
		WillReturnRows(firstStats)
	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoEvmTxEdgeBackfill), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// Second iteration: no pending rows, mark completion
	expectSeqInfoLookup(mock, types.SeqInfoEvmTxEdgeBackfill, 42)
	mock.ExpectBegin()
	secondStats := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(0), nil, int64(0))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(42), defaultBatchSize).
		WillReturnRows(secondStats)
	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoEvmTxEdgeBackfill), int64(-1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, ext.Run(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunPropagatesEvmInternalBackfillError(t *testing.T) {
	logger := newTestLogger()
	db, mock, err := testutil.NewMockDB(logger)
	require.NoError(t, err)

	ext := &Extension{
		logger:             logger,
		db:                 db,
		batchSize:          defaultBatchSize,
		idleDelay:          time.Millisecond,
		hasEvmInternalData: true,
		cfg:                &config.Config{},
	}

	boom := fmt.Errorf("evm internal failure")
	expectSeqInfoLookup(mock, types.SeqInfoEvmInternalTxEdgeBackfill, 0)
	mock.ExpectBegin()
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(0), defaultBatchSize).
		WillReturnError(boom)
	mock.ExpectRollback()

	err = ext.Run(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, boom)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunProcessesEvmInternalBackfillToCompletion(t *testing.T) {
	logger := newTestLogger()
	db, mock, err := testutil.NewMockDB(logger)
	require.NoError(t, err)

	ext := &Extension{
		logger:             logger,
		db:                 db,
		batchSize:          defaultBatchSize,
		idleDelay:          time.Millisecond,
		hasEvmInternalData: true,
		hasTxTables:        false,
		hasEvmTables:       false,
		cfg:                &config.Config{},
	}

	expectSeqInfoLookup(mock, types.SeqInfoEvmInternalTxEdgeBackfill, 0)
	mock.ExpectBegin()
	firstStats := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(3), int64(77), int64(3))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(0), defaultBatchSize).
		WillReturnRows(firstStats)
	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoEvmInternalTxEdgeBackfill), int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	expectSeqInfoLookup(mock, types.SeqInfoEvmInternalTxEdgeBackfill, 77)
	mock.ExpectBegin()
	secondStats := sqlmock.NewRows([]string{
		"pending_count",
		"max_sequence",
		"accounts_inserted",
	}).AddRow(int64(0), nil, int64(0))
	mock.ExpectQuery(`WITH pending AS \(`).
		WithArgs(int64(77), defaultBatchSize).
		WillReturnRows(secondStats)
	mock.ExpectExec(`INSERT INTO "seq_info" \("name","sequence"\) VALUES \(\$1,\$2\) ON CONFLICT \("name"\) DO UPDATE SET`).
		WithArgs(string(types.SeqInfoEvmInternalTxEdgeBackfill), int64(-1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, ext.Run(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}
