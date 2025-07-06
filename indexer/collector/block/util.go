package block

import (
	"encoding/base64"

	cbjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/unknownproto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func getBlock(chainId string, height int64, tx *gorm.DB) (block types.CollectedBlock, err error) {
	if err := tx.Where("chain_id = ? AND height = ?", chainId, height).First(&block).Error; err != nil {
		return block, err
	}

	return block, nil
}

func getTotalFee(txs []string, cdc codec.Codec) (fee []byte, err error) {
	var feeCoins sdk.Coins

	for _, txRaw := range txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return fee, err
		}

		var raw sdktx.TxRaw
		if err = unknownproto.RejectUnknownFieldsStrict(txByte, &raw, cdc.InterfaceRegistry()); err != nil {
			return fee, err
		}

		if err = cdc.Unmarshal(txByte, &raw); err != nil {
			return fee, err
		}

		var authInfo sdktx.AuthInfo
		if err = unknownproto.RejectUnknownFieldsStrict(raw.AuthInfoBytes, &authInfo, cdc.InterfaceRegistry()); err != nil {
			return fee, err
		}

		if err = cdc.Unmarshal(raw.AuthInfoBytes, &authInfo); err != nil {
			return fee, err
		}

		feeCoins = feeCoins.Add(authInfo.Fee.Amount...)
	}

	return cbjson.Marshal(feeCoins)
}
