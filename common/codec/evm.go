//go:build evm
// +build evm

package codec

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/initia-labs/minievm/app"
)

var (
    TxConfig client.TxConfig
    Cdc      codec.Codec
)

func init() {
	cfg := app.MakeEncodingConfig()
	TxConfig = cfg.TxConfig
	Cdc = cfg.Codec
	sdkConfig := sdk.GetConfig()
	sdkConfig.SetCoinType(app.CoinType)
	accountPubKeyPrefix := app.AccountAddressPrefix + "pub"
	validatorAddressPrefix := app.AccountAddressPrefix + "valoper"
	validatorPubKeyPrefix := app.AccountAddressPrefix + "valoperpub"
	consNodeAddressPrefix := app.AccountAddressPrefix + "valcons"
	consNodePubKeyPrefix := app.AccountAddressPrefix + "valconspub"

	sdkConfig.SetBech32PrefixForAccount(app.AccountAddressPrefix, accountPubKeyPrefix)
	sdkConfig.SetBech32PrefixForValidator(validatorAddressPrefix, validatorPubKeyPrefix)
	sdkConfig.SetBech32PrefixForConsensusNode(consNodeAddressPrefix, consNodePubKeyPrefix)
	sdkConfig.SetAddressVerifier(app.VerifyAddressLen())
	sdkConfig.Seal()
}
