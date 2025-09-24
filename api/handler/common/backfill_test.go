package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/initia-labs/rollytics/types"
)

func setupBackfillTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`CREATE TABLE seq_info (
		name TEXT PRIMARY KEY,
		sequence INTEGER
	)`).Error)

	return db
}

func TestGetEdgeBackfillStatus(t *testing.T) {
	db := setupBackfillTestDB(t)

	status, err := GetEdgeBackfillStatus(db, types.SeqInfoTxEdgeBackfill)
	require.NoError(t, err)
	require.False(t, status.Completed)
	require.Equal(t, int64(0), status.Sequence)

	require.NoError(t, db.Exec(`INSERT INTO seq_info (name, sequence) VALUES (?, ?)`, string(types.SeqInfoTxEdgeBackfill), 5).Error)

	status, err = GetEdgeBackfillStatus(db, types.SeqInfoTxEdgeBackfill)
	require.NoError(t, err)
	require.False(t, status.Completed)
	require.Equal(t, int64(5), status.Sequence)

	require.NoError(t, db.Exec(`UPDATE seq_info SET sequence = -1 WHERE name = ?`, string(types.SeqInfoTxEdgeBackfill)).Error)

	status, err = GetEdgeBackfillStatus(db, types.SeqInfoTxEdgeBackfill)
	require.NoError(t, err)
	require.True(t, status.Completed)
	require.Equal(t, int64(-1), status.Sequence)
}

func TestIsEdgeBackfillReady(t *testing.T) {
	db := setupBackfillTestDB(t)

	ready, err := IsEdgeBackfillReady(db, types.SeqInfoTxEdgeBackfill)
	require.NoError(t, err)
	require.False(t, ready)

	require.NoError(t, db.Exec(`INSERT INTO seq_info (name, sequence) VALUES (?, ?)`, string(types.SeqInfoTxEdgeBackfill), -1).Error)

	ready, err = IsEdgeBackfillReady(db, types.SeqInfoTxEdgeBackfill)
	require.NoError(t, err)
	require.True(t, ready)
}
