package collector

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/initia-labs/initia/app/params"
	cryptocodec "github.com/initia-labs/initia/crypto/codec"
	"github.com/initia-labs/rollytics/config"
)

var cdc codec.Codec

func init() {
	encodingConfig := params.MakeEncodingConfig()
	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	cryptocodec.RegisterLegacyAminoCodec(encodingConfig.Amino)
	cryptocodec.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	cdc = encodingConfig.Codec

	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	prefix := cfg.GetChainConfig().AccountAddressPrefix
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
