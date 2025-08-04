package evm_nft

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

const (
	nftTopic  = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	emptyAddr = "0x0000000000000000000000000000000000000000000000000000000000000000"
)

func isEvmNftLog(log evmtypes.Log) bool {
	return len(log.Topics) == 4 && log.Topics[0] == nftTopic && log.Data == "0x"
}

func convertHexStringToDecString(hex string) (string, error) {
	hex = strings.TrimPrefix(hex, "0x")
	bi, ok := new(big.Int).SetString(hex, 16)
	if !ok {
		return "", errors.New("failed to convert hex to dec")
	}
	return bi.String(), nil
}

func getCollectionCreationInfo(addr string, tx *gorm.DB) ([]byte, int64, error) {
	bechAddr, err := util.AccAddressFromString(addr)
	if err != nil {
		return nil, 0, err
	}

	var accountDict types.CollectedAccountDict
	if err := tx.Where("account = ?", bechAddr).First(&accountDict).Error; err != nil {
		return nil, 0, err
	}

	var ctx types.CollectedTx
	if err := tx.
		Where("account_ids && ?", pq.Array([]int64{accountDict.Id})).
		Order("sequence ASC").
		Limit(1).
		First(&ctx).Error; err != nil {
		return nil, 0, err
	}

	var signerAccount types.CollectedAccountDict
	if err := tx.Where("id = ?", ctx.SignerId).First(&signerAccount).Error; err != nil {
		return nil, 0, err
	}

	return signerAccount.Account, ctx.Height, nil
}

func isEvmRevertError(err error) bool {
	errString := fmt.Sprintf("%+v", err)
	return strings.Contains(errString, "Reverted")
}
