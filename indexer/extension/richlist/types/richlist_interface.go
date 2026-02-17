package types

import (
	"context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"

	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	rollytypes "github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/querier"
)

type RichListProcessor interface {
	ProcessBalanceChanges(ctx context.Context, txs []rollytypes.CollectedTx, moduleAccounts []sdk.AccAddress) map[richlistutils.BalanceChangeKey]sdkmath.Int
	AfterProcess(ctx context.Context, dbTx *gorm.DB, currentHeight int64, negativeDenoms []string, q *querier.Querier) error
}
