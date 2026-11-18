package v1_0_2

import (
	"errors"
	"io"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	abci "github.com/cometbft/cometbft/abci/types"
	cbjson "github.com/cometbft/cometbft/libs/json"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/orm/testutil"
	"github.com/initia-labs/rollytics/types"
)

func TestPatchMoveNFTUsesArchivedEvents(t *testing.T) {
	queryErr := errors.New("transaction query failed")
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "replays without account or type-tag edges"},
		{name: "query error preserves existing NFTs", err: queryErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			db, mock, err := testutil.NewMockDB(logger)
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			cfg := &config.Config{}
			cfg.SetDBConfig(&dbconfig.Config{BatchSize: 10})

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "nft"`)).
				WillReturnRows(sqlmock.NewRows([]string{"addr", "token_id"}).AddRow([]byte{1}, "nft"))
			// Each predicate matches a complete Move event and its type_tag attribute.
			// No account dictionary or edge tables are needed to select replay data.
			query := mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "tx" WHERE data @> $1 OR data @> $2 OR data @> $3 OR data @> $4 ORDER BY sequence ASC`)).
				WithArgs(
					`{"events":[{"type":"move","attributes":[{"key":"type_tag","value":"0x1::collection::MintEvent"}]}]}`,
					`{"events":[{"type":"move","attributes":[{"key":"type_tag","value":"0x1::object::TransferEvent"}]}]}`,
					`{"events":[{"type":"move","attributes":[{"key":"type_tag","value":"0x1::nft::MutationEvent"}]}]}`,
					`{"events":[{"type":"move","attributes":[{"key":"type_tag","value":"0x1::collection::BurnEvent"}]}]}`,
				)
			if tc.err != nil {
				query.WillReturnError(tc.err)
			} else {
				mintTime, mutationTime := time.Unix(10, 0).UTC(), time.Unix(11, 0).UTC()
				mint := archivedMoveEvent(t, "0x1::collection::MintEvent", `{"collection":"0x02","nft":"0x01"}`, mintTime)
				mutation := archivedMoveEvent(t, "0x1::nft::MutationEvent", `{"nft":"0x01","mutated_field_name":"uri","new_value":"ipfs://updated"}`, mutationTime)
				query.WillReturnRows(sqlmock.NewRows([]string{"height", "sequence", "data"}).
					AddRow(10, 100, mint).AddRow(11, 101, mutation))
				mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM nft`)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "nft_collection" WHERE addr = $1`)).
					WithArgs([]byte{2}, 1).
					WillReturnRows(sqlmock.NewRows([]string{"addr", "creator_id"}).AddRow([]byte{2}, 42))
				mock.ExpectBegin()
				mock.ExpectExec(`INSERT INTO "nft"`).
					WithArgs([]byte{2}, "0x01", []byte{1}, 10, mintTime, 42, "").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta(`UPDATE "nft" SET "height"=$1,"timestamp"=$2,"uri"=$3 WHERE addr = $4`)).
					WithArgs(11, mutationTime, "ipfs://updated", []byte{1}).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "nft"`)).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			}

			err = PatchMoveNFT(db.DB, cfg, logger)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.ErrorContains(t, err, "failed to query NFT transactions")
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func archivedMoveEvent(t *testing.T, typeTag, data string, timestamp time.Time) []byte {
	t.Helper()
	// Match the Comet JSON serialization used by the transaction collector.
	events, err := cbjson.Marshal([]abci.Event{{Type: "move", Attributes: []abci.EventAttribute{
		{Key: "type_tag", Value: typeTag},
		{Key: "data", Value: data},
	}}})
	require.NoError(t, err)
	txData, err := cbjson.Marshal(types.Tx{Timestamp: timestamp, Events: events})
	require.NoError(t, err)
	return txData
}
