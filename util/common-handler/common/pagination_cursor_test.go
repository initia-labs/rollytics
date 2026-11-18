package common

import (
	"encoding/base64"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/types"
)

func TestParsePaginationKeyHeightValidation(t *testing.T) {
	for _, value := range []string{"null", `"123"`, "true", "{}", "[]", "1.5", "1e30"} {
		t.Run(value, func(t *testing.T) {
			key := base64.StdEncoding.EncodeToString([]byte(`{"height":` + value + `}`))
			err := parsePaginationKey(key, &Pagination{CursorType: CursorTypeOffset})
			require.Error(t, err)
			require.Contains(t, err.Error(), "height")
		})
	}
}

func TestParsePaginationKeyPreservesCompositeFallback(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	pagination := &Pagination{CursorType: CursorTypeOffset, Limit: 20, Offset: 40, Order: OrderDesc}
	key := base64.StdEncoding.EncodeToString([]byte(`{"height":null,"token_id":"nft"}`))
	require.NoError(t, parsePaginationKey(key, pagination))
	require.Equal(t, CursorTypeComposite, pagination.CursorType)

	mock.ExpectQuery(`SELECT \* FROM "nft" ORDER BY height DESC, token_id DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 40).
		WillReturnRows(sqlmock.NewRows([]string{"height", "token_id"}))
	var nfts []types.CollectedNft
	require.NoError(t, pagination.ApplyToNft(db.Model(&types.CollectedNft{}), "height").Find(&nfts).Error)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestParsePaginationKeyPreservesValidCursors(t *testing.T) {
	tests := []struct {
		cursor     string
		cursorType CursorType
		offset     int
	}{
		{`{"height":123}`, CursorTypeHeight, 0},
		{`{"height":0}`, CursorTypeHeight, 0},
		{`{"height":123,"token_id":"nft"}`, CursorTypeComposite, 0},
		{`{"sequence":456}`, CursorTypeSequence, 0},
		{`{"sequence":456,"height":123}`, CursorTypeSequence, 0},
		{"100", CursorTypeOffset, 100},
	}
	for _, tt := range tests {
		t.Run(tt.cursor, func(t *testing.T) {
			pagination := &Pagination{CursorType: CursorTypeOffset}
			key := base64.StdEncoding.EncodeToString([]byte(tt.cursor))
			require.NoError(t, parsePaginationKey(key, pagination))
			require.Equal(t, tt.cursorType, pagination.CursorType)
			require.Equal(t, tt.offset, pagination.Offset)
			if tt.cursorType == CursorTypeHeight || tt.cursorType == CursorTypeComposite {
				_, err := pagination.safeGetInt64("height")
				require.NoError(t, err)
			}
		})
	}
}
