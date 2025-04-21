package block

import (
	"encoding/base64"

	cbjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func getBlock(chainId string, height int64, tx *gorm.DB) (block types.CollectedBlock, err error) {
	res := tx.Where("chain_id = ? AND height = ?", chainId, height).Take(&block)
	return block, res.Error
}

func getTotalFee(txs []string, txConfig client.TxConfig) (fee []byte, err error) {
	var feeCoins sdk.Coins
	txDecoder := txConfig.TxDecoder()

	for txIndex, txRaw := range txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return fee, err
		}

		decoded, err := txDecoder(txByte)
		if err != nil {
			if txIndex == 0 {
				continue
			}
			return nil, err
		}

		f := decoded.(sdk.FeeTx).GetFee()
		feeCoins = feeCoins.Add(f...)
	}

	return cbjson.Marshal(feeCoins)
}
