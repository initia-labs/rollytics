package common

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func TestExactCountsSkipWorkWhileBusyAndRecover(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.MatchExpectationsInOrder(false)
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var wg sync.WaitGroup
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		wg.Wait()
	}()
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:block_count", func(_ *gorm.DB) {
		entered <- struct{}{}
		<-release
	}))
	for range 4 {
		mock.ExpectBegin()
		mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))
		mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()
	}
	type result struct {
		total int64
		err   error
	}
	results := make(chan result, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total, err := GetCountWithTimeout(db.Model(&types.CollectedTx{}), true)
			results <- result{total, err}
		}()
	}
	for range 4 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("count query did not start")
		}
	}
	// A separate mock has no SQL expectations: saturation must not touch it.
	otherDB, otherMock, otherCleanup := setupMockDB(t)
	defer otherCleanup()
	total, err := GetCountWithTimeout(otherDB.Model(&types.CollectedTx{}), true)
	require.NoError(t, err)
	require.Equal(t, int64(-1), total)
	total, err = GetCountWithTimeout(otherDB.Model(&types.CollectedTx{}), false)
	require.NoError(t, err)
	require.Zero(t, total)
	require.NoError(t, otherMock.ExpectationsWereMet())
	close(release)
	for range 4 {
		res := <-results
		require.NoError(t, res.err)
		require.Equal(t, int64(42), res.total)
	}
	require.NoError(t, mock.ExpectationsWereMet())
	otherMock.ExpectBegin()
	otherMock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
	otherMock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(17))
	otherMock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	otherMock.ExpectCommit()
	total, err = GetCountWithTimeout(otherDB.Model(&types.CollectedTx{}), true)
	require.NoError(t, err)
	require.Equal(t, int64(17), total, "exact counts must resume after capacity is released")
	require.NoError(t, otherMock.ExpectationsWereMet())
}

func TestCountTimeoutPreservesOuterPageTransaction(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	outer := db.Begin()
	require.NoError(t, outer.Error)
	mock.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
		WillReturnError(errors.New("canceling statement due to statement timeout"))
	mock.ExpectExec(`ROLLBACK TO SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
	total, err := GetCountWithTimeout(outer.Model(&types.CollectedTx{}), true)
	require.NoError(t, err)
	require.Equal(t, int64(-1), total)
	mock.ExpectQuery(`SELECT \* FROM "tx" LIMIT \$1`).WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"sequence"}).AddRow(9))
	var page []types.CollectedTx
	require.NoError(t, outer.Model(&types.CollectedTx{}).Limit(1).Find(&page).Error)
	require.Len(t, page, 1)
	mock.ExpectRollback()
	require.NoError(t, outer.Rollback().Error)
	require.NoError(t, mock.ExpectationsWereMet())
}
