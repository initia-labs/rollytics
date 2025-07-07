package evm_nft

import (
	"errors"
	"math/big"
	"strings"

	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"gorm.io/gorm"

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

func getCollectionCreator(addr string, tx *gorm.DB) (string, error) {
	bechAddr, err := util.AccAddressFromString(addr)
	if err != nil {
		return "", err
	}

	var result struct {
		Signer string
	}

	err = tx.
		Table("account_tx AS a").
		Select("t.signer").
		Joins("JOIN tx AS t ON t.chain_id = a.chain_id AND t.hash = a.hash").
		Where("a.account = ?", bechAddr.String()).
		Order("t.sequence ASC").
		Limit(1).
		Scan(&result).Error

	return result.Signer, err
}
