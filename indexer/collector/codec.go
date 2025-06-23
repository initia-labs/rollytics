package collector

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/initia-labs/initia/app/params"
	cryptocodec "github.com/initia-labs/initia/crypto/codec"
)

func getCodec() codec.Codec {
	encodingConfig := params.MakeEncodingConfig()
	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	cryptocodec.RegisterLegacyAminoCodec(encodingConfig.Amino)
	cryptocodec.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	return encodingConfig.Codec
}
