//go:build wasmvm
// +build wasmvm

package codec

import (
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/initia-labs/miniwasm/app"
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
	sdkConfig.SetAddressVerifier(wasmtypes.VerifyAddressLen())
	sdkConfig.Seal()
}
