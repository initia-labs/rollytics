package main

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/initia-labs/initia/app"
	"github.com/initia-labs/rollytics/indexer/cmd"
)

func init() {
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

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
