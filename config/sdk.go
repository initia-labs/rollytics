package config

import (
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var sdkConfigOnce sync.Once

// InitializeSDKConfig initializes the SDK configuration with chain-specific settings
func InitializeSDKConfig(accountAddressPrefix string) {
	sdkConfigOnce.Do(func() {
		prefix := accountAddressPrefix
		accountPubKeyPrefix := prefix + "pub"
		validatorAddressPrefix := prefix + "valoper"
		validatorPubKeyPrefix := prefix + "valoperpub"
		consNodeAddressPrefix := prefix + "valcons"
		consNodePubKeyPrefix := prefix + "valconspub"

		sdkConfig := sdk.GetConfig()
		sdkConfig.SetBech32PrefixForAccount(prefix, accountPubKeyPrefix)
		sdkConfig.SetBech32PrefixForValidator(validatorAddressPrefix, validatorPubKeyPrefix)
		sdkConfig.SetBech32PrefixForConsensusNode(consNodeAddressPrefix, consNodePubKeyPrefix)
		sdkConfig.SetAddressVerifier(verifyAddressLen())
		sdkConfig.Seal()
	})
}

func verifyAddressLen() func(addr []byte) error {
	return func(addr []byte) error {
		addrLen := len(addr)
		if addrLen != 20 && addrLen != 32 {
			return sdkerrors.ErrInvalidAddress
		}

		return nil
	}
}
