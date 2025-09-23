package txedgebackfill

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	exttypes "github.com/initia-labs/rollytics/indexer/extension/types"
	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

const (
	extensionName       = "tx-edge-backfill"
	defaultBatchSize    = 2000
	defaultIdleDuration = 5 * time.Second
)

const (
	txBatchSQL = `
WITH pending AS (
	SELECT t.sequence,
	       t.signer_id,
	       COALESCE(t.account_ids, '{}'::bigint[]) AS account_ids,
	       COALESCE(t.msg_type_ids, '{}'::bigint[]) AS msg_type_ids,
	       COALESCE(t.type_tag_ids, '{}'::bigint[]) AS type_tag_ids,
	       COALESCE(t.nft_ids, '{}'::bigint[]) AS nft_ids
	FROM tx t
	WHERE t.sequence > ?
	  AND (
		((COALESCE(array_length(t.account_ids, 1), 0) > 0 OR t.signer_id IS NOT NULL)
			AND NOT EXISTS (SELECT 1 FROM tx_accounts ta WHERE ta.sequence = t.sequence))
		OR (COALESCE(array_length(t.msg_type_ids, 1), 0) > 0
			AND NOT EXISTS (SELECT 1 FROM tx_msg_types tm WHERE tm.sequence = t.sequence))
		OR (COALESCE(array_length(t.type_tag_ids, 1), 0) > 0
			AND NOT EXISTS (SELECT 1 FROM tx_type_tags tt WHERE tt.sequence = t.sequence))
		OR (COALESCE(array_length(t.nft_ids, 1), 0) > 0
			AND NOT EXISTS (SELECT 1 FROM tx_nfts tn WHERE tn.sequence = t.sequence))
	  )
	ORDER BY t.sequence ASC
	LIMIT ?
),
insert_accounts AS (
	INSERT INTO tx_accounts (account_id, sequence, signer)
	SELECT account_id, sequence, signer
	FROM (
		SELECT p.sequence,
		       acct_id.account_id,
		       (acct_id.account_id = p.signer_id) AS signer
		FROM pending p
		CROSS JOIN LATERAL UNNEST(p.account_ids) AS acct_id(account_id)
		WHERE acct_id.account_id IS NOT NULL
		UNION
		SELECT p.signer_id AS account_id,
		       p.sequence,
		       TRUE AS signer
		FROM pending p
		WHERE p.signer_id IS NOT NULL
	) all_accounts
	WHERE NOT EXISTS (
		SELECT 1 FROM tx_accounts ta WHERE ta.sequence = all_accounts.sequence
	)
	ON CONFLICT DO NOTHING
	RETURNING sequence
),
insert_msg_types AS (
	INSERT INTO tx_msg_types (msg_type_id, sequence)
	SELECT DISTINCT msg.msg_type_id, p.sequence
	FROM pending p
	CROSS JOIN LATERAL UNNEST(p.msg_type_ids) AS msg(msg_type_id)
	WHERE msg.msg_type_id IS NOT NULL
	  AND NOT EXISTS (SELECT 1 FROM tx_msg_types tm WHERE tm.sequence = p.sequence)
	ON CONFLICT DO NOTHING
	RETURNING sequence
),
insert_type_tags AS (
	INSERT INTO tx_type_tags (type_tag_id, sequence)
	SELECT DISTINCT tag.type_tag_id, p.sequence
	FROM pending p
	CROSS JOIN LATERAL UNNEST(p.type_tag_ids) AS tag(type_tag_id)
	WHERE tag.type_tag_id IS NOT NULL
	  AND NOT EXISTS (SELECT 1 FROM tx_type_tags tt WHERE tt.sequence = p.sequence)
	ON CONFLICT DO NOTHING
	RETURNING sequence
),
insert_nfts AS (
	INSERT INTO tx_nfts (nft_id, sequence)
	SELECT DISTINCT nft.nft_id, p.sequence
	FROM pending p
	CROSS JOIN LATERAL UNNEST(p.nft_ids) AS nft(nft_id)
	WHERE nft.nft_id IS NOT NULL
	  AND NOT EXISTS (SELECT 1 FROM tx_nfts tn WHERE tn.sequence = p.sequence)
	ON CONFLICT DO NOTHING
	RETURNING sequence
)
SELECT
	(SELECT COUNT(*) FROM pending) AS pending_count,
	(SELECT MAX(sequence) FROM pending) AS max_sequence,
	(SELECT COUNT(*) FROM insert_accounts) AS accounts_inserted,
	(SELECT COUNT(*) FROM insert_msg_types) AS msg_types_inserted,
	(SELECT COUNT(*) FROM insert_type_tags) AS type_tags_inserted,
	(SELECT COUNT(*) FROM insert_nfts) AS nfts_inserted;`

	evmTxBatchSQL = `
WITH pending AS (
	SELECT t.sequence,
	       t.signer_id,
	       COALESCE(t.account_ids, '{}'::bigint[]) AS account_ids
	FROM evm_tx t
	WHERE t.sequence > ?
	  AND (COALESCE(array_length(t.account_ids, 1), 0) > 0 OR t.signer_id IS NOT NULL)
	  AND NOT EXISTS (SELECT 1 FROM evm_tx_accounts eta WHERE eta.sequence = t.sequence)
	ORDER BY t.sequence ASC
	LIMIT ?
),
insert_accounts AS (
	INSERT INTO evm_tx_accounts (account_id, sequence, signer)
	SELECT account_id, sequence, signer
	FROM (
		SELECT p.sequence,
		       acct_id.account_id,
		       (acct_id.account_id = p.signer_id) AS signer
		FROM pending p
		CROSS JOIN LATERAL UNNEST(p.account_ids) AS acct_id(account_id)
		WHERE acct_id.account_id IS NOT NULL
		UNION
		SELECT p.signer_id AS account_id,
		       p.sequence,
		       TRUE AS signer
		FROM pending p
		WHERE p.signer_id IS NOT NULL
	) all_accounts
	WHERE NOT EXISTS (
		SELECT 1 FROM evm_tx_accounts eta WHERE eta.sequence = all_accounts.sequence
	)
	ON CONFLICT DO NOTHING
	RETURNING sequence
)
SELECT
	(SELECT COUNT(*) FROM pending) AS pending_count,
	(SELECT MAX(sequence) FROM pending) AS max_sequence,
	(SELECT COUNT(*) FROM insert_accounts) AS accounts_inserted;`

	evmInternalBatchSQL = `
WITH pending AS (
	SELECT t.sequence,
	       COALESCE(t.account_ids, '{}'::bigint[]) AS account_ids
	FROM evm_internal_tx t
	WHERE t.sequence > ?
	  AND COALESCE(array_length(t.account_ids, 1), 0) > 0
	  AND NOT EXISTS (SELECT 1 FROM evm_internal_tx_accounts eita WHERE eita.sequence = t.sequence)
	ORDER BY t.sequence ASC
	LIMIT ?
),
insert_accounts AS (
	INSERT INTO evm_internal_tx_accounts (account_id, sequence)
	SELECT DISTINCT acct.account_id, p.sequence
	FROM pending p
	CROSS JOIN LATERAL UNNEST(p.account_ids) AS acct(account_id)
	WHERE acct.account_id IS NOT NULL
	  AND NOT EXISTS (SELECT 1 FROM evm_internal_tx_accounts eita WHERE eita.sequence = p.sequence)
	ON CONFLICT DO NOTHING
	RETURNING sequence
)
SELECT
	(SELECT COUNT(*) FROM pending) AS pending_count,
	(SELECT MAX(sequence) FROM pending) AS max_sequence,
	(SELECT COUNT(*) FROM insert_accounts) AS accounts_inserted;`
)

var _ exttypes.Extension = (*Extension)(nil)

// Extension streams existing transactions into the new edge tables while the
// indexer keeps running.
type Extension struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *orm.Database

	batchSize int
	idleDelay time.Duration

	hasTxTables        bool
	hasEvmTables       bool
	hasEvmInternalData bool
}

// New constructs the edge backfill extension if the necessary tables exist. If
// the migration hasn't yet been applied, the extension is skipped gracefully.
func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) exttypes.Extension {
	migrator := db.Migrator()

	hasTxEdges := migrator.HasTable(types.CollectedTxAccount{}.TableName()) &&
		migrator.HasColumn(types.CollectedTx{}.TableName(), "account_ids")

	hasEvmTxEdges := migrator.HasTable(types.CollectedEvmTxAccount{}.TableName()) &&
		migrator.HasColumn(types.CollectedEvmTx{}.TableName(), "account_ids")

	hasEvmInternalEdges := migrator.HasTable(types.CollectedEvmInternalTxAccount{}.TableName()) &&
		migrator.HasColumn(types.CollectedEvmInternalTx{}.TableName(), "account_ids")

	if !hasTxEdges && !hasEvmTxEdges && !hasEvmInternalEdges {
		return nil
	}

	readyTx := !hasTxEdges
	if hasTxEdges {
		if ok, err := isBackfillReady(db.DB, types.SeqInfoTxEdgeBackfill); err != nil {
			logger.Warn("failed to read tx edge backfill status", slog.Any("error", err))
		} else {
			readyTx = ok
		}
	}

	readyEvmTx := !hasEvmTxEdges
	if hasEvmTxEdges {
		if ok, err := isBackfillReady(db.DB, types.SeqInfoEvmTxEdgeBackfill); err != nil {
			logger.Warn("failed to read evm tx edge backfill status", slog.Any("error", err))
		} else {
			readyEvmTx = ok
		}
	}

	readyEvmInternal := !hasEvmInternalEdges
	if hasEvmInternalEdges {
		if ok, err := isBackfillReady(db.DB, types.SeqInfoEvmInternalTxEdgeBackfill); err != nil {
			logger.Warn("failed to read evm internal tx edge backfill status", slog.Any("error", err))
		} else {
			readyEvmInternal = ok
		}
	}

	if readyTx && readyEvmTx && readyEvmInternal {
		logger.Info("edge backfill already completed; skipping worker")
		return nil
	}

	return &Extension{
		cfg:                cfg,
		logger:             logger.With("extension", extensionName),
		db:                 db,
		batchSize:          defaultBatchSize,
		idleDelay:          defaultIdleDuration,
		hasTxTables:        hasTxEdges,
		hasEvmTables:       hasEvmTxEdges,
		hasEvmInternalData: hasEvmInternalEdges,
	}
}

// Name returns the extension identifier.
func (e *Extension) Name() string {
	return extensionName
}

// Run begins the background backfill loop. It processes batches until no work
// remains, then sleeps briefly before checking again.
func (e *Extension) Run(ctx context.Context) error {
	if !e.hasTxTables && !e.hasEvmTables && !e.hasEvmInternalData {
		return nil
	}

	e.logger.Info("tx edge backfill worker started",
		slog.Int("batch_size", e.batchSize),
		slog.Duration("idle_delay", e.idleDelay))

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("tx edge backfill worker shutting down",
				slog.String("reason", ctx.Err().Error()))
			return nil
		default:
		}

		processedAny := false
		allComplete := true

		if e.hasTxTables {
			didWork, complete, err := e.backfillTx(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				e.logger.Error("tx backfill failed", slog.Any("error", err))
				return err
			}
			processedAny = processedAny || didWork
			allComplete = allComplete && complete
		}

		if e.hasEvmTables {
			didWork, complete, err := e.backfillEvmTx(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				e.logger.Error("evm tx backfill failed", slog.Any("error", err))
				return err
			}
			processedAny = processedAny || didWork
			allComplete = allComplete && complete
		}

		if e.hasEvmInternalData {
			didWork, complete, err := e.backfillEvmInternalTx(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				e.logger.Error("evm internal tx backfill failed", slog.Any("error", err))
				return err
			}
			processedAny = processedAny || didWork
			allComplete = allComplete && complete
		}

		if !processedAny {
			if allComplete {
				e.logger.Info("edge backfill completed; exiting worker")
				return nil
			}
			select {
			case <-ctx.Done():
				e.logger.Info("tx edge backfill worker shutting down",
					slog.String("reason", ctx.Err().Error()))
				return nil
			case <-time.After(e.idleDelay):
			}
		}
	}
}

type txBatchStats struct {
	PendingCount     int64
	MaxSequence      sql.NullInt64
	AccountsInserted int64
	MsgTypesInserted int64
	TypeTagsInserted int64
	NftsInserted     int64
}

type simpleBatchStats struct {
	PendingCount     int64
	MaxSequence      sql.NullInt64
	AccountsInserted int64
}

func (e *Extension) backfillTx(ctx context.Context) (bool, bool, error) {
	session := e.db.WithContext(ctx)

	seqInfo, err := indexerutil.GetSeqInfo(types.SeqInfoTxEdgeBackfill, session)
	if err != nil {
		return false, false, err
	}
	if seqInfo.Sequence == -1 {
		return false, true, nil
	}

	stats := txBatchStats{}

	if err := session.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(txBatchSQL, seqInfo.Sequence, e.batchSize).Scan(&stats).Error; err != nil {
			return err
		}
		if stats.PendingCount == 0 {
			seqInfo.Sequence = -1 // flag completion
		} else {
			if !stats.MaxSequence.Valid {
				return errors.New("tx backfill returned pending rows without max sequence")
			}
			seqInfo.Sequence = stats.MaxSequence.Int64
		}
		return tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo).Error
	}); err != nil {
		return false, false, err
	}

	if stats.PendingCount == 0 {
		return false, true, nil
	}

	return true, false, nil
}

func (e *Extension) backfillEvmTx(ctx context.Context) (bool, bool, error) {
	session := e.db.WithContext(ctx)

	seqInfo, err := indexerutil.GetSeqInfo(types.SeqInfoEvmTxEdgeBackfill, session)
	if err != nil {
		return false, false, err
	}
	if seqInfo.Sequence == -1 {
		return false, true, nil
	}

	stats := simpleBatchStats{}

	if err := session.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(evmTxBatchSQL, seqInfo.Sequence, e.batchSize).Scan(&stats).Error; err != nil {
			return err
		}
		if stats.PendingCount == 0 {
			seqInfo.Sequence = -1 // flag completion
		} else {
			if !stats.MaxSequence.Valid {
				return errors.New("evm tx backfill returned pending rows without max sequence")
			}
			seqInfo.Sequence = stats.MaxSequence.Int64
		}
		return tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo).Error
	}); err != nil {
		return false, false, err
	}

	if stats.PendingCount == 0 {
		return false, true, nil
	}

	return true, false, nil
}

func (e *Extension) backfillEvmInternalTx(ctx context.Context) (bool, bool, error) {
	session := e.db.WithContext(ctx)

	seqInfo, err := indexerutil.GetSeqInfo(types.SeqInfoEvmInternalTxEdgeBackfill, session)
	if err != nil {
		return false, false, err
	}
	if seqInfo.Sequence == -1 {
		return false, true, nil
	}

	stats := simpleBatchStats{}

	if err := session.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(evmInternalBatchSQL, seqInfo.Sequence, e.batchSize).Scan(&stats).Error; err != nil {
			return err
		}
		if stats.PendingCount == 0 {
			seqInfo.Sequence = -1 // flag completion
		} else {
			if !stats.MaxSequence.Valid {
				return errors.New("evm internal tx backfill returned pending rows without max sequence")
			}
			seqInfo.Sequence = stats.MaxSequence.Int64
		}
		return tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo).Error
	}); err != nil {
		return false, false, err
	}

	if stats.PendingCount == 0 {
		return false, true, nil
	}

	return true, false, nil
}

func isBackfillReady(db *gorm.DB, name types.SeqInfoName) (bool, error) {
	var info types.CollectedSeqInfo
	if err := db.Where("name = ?", name).First(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return info.Sequence == -1, nil
}
