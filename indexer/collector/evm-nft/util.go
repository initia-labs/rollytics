package evm_nft

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func isEvmNftLog(log evmtypes.Log) bool {
	return len(log.Topics) == 4 && log.Topics[0] == types.EvmTransferTopic && log.Data == "0x"
}

func convertHexStringToDecString(hex string) (string, error) {
	hex = strings.TrimPrefix(hex, "0x")
	bi, ok := new(big.Int).SetString(hex, 16)
	if !ok {
		return "", errors.New("failed to convert hex to dec")
	}
	return bi.String(), nil
}

func getCollectionCreationInfo(chainId, addr string, tx *gorm.DB) (*CollectionCreationInfo, error) {
	bechAddr, err := util.AccAddressFromString(addr)
	if err != nil {
		return nil, err
	}

	var accountDict types.CollectedAccountDict
	if err := tx.Where("account = ?", bechAddr).First(&accountDict).Error; err != nil {
		return nil, err
	}

	var ctxa types.CollectedTxAccount
	if err := tx.
		Where("account_id = ?", accountDict.Id).
		Order("sequence ASC").
		Limit(1).
		First(&ctxa).Error; err != nil {
		return nil, err
	}

	var ctx types.CollectedTx
	if err := tx.
		Where("sequence = ?", ctxa.Sequence).
		Limit(1).
		First(&ctx).Error; err != nil {
		return nil, err
	}

	var signerAccount types.CollectedAccountDict
	if err := tx.Where("id = ?", ctx.SignerId).First(&signerAccount).Error; err != nil {
		return nil, err
	}

	height := ctx.Height
	var block types.CollectedBlock
	if err := tx.Where("chain_id = ? AND height = ?", chainId, height).
		First(&block).Error; err != nil {
		return nil, err
	}

	return &CollectionCreationInfo{
		Creator:   sdk.AccAddress(signerAccount.Account).String(),
		Height:    ctx.Height,
		Timestamp: block.Timestamp,
	}, nil
}

func isEvmRevertError(err error) bool {
	errString := fmt.Sprintf("%+v", err)
	return strings.Contains(errString, "Reverted")
}
