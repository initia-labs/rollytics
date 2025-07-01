package common

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/util"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func GetMsgsParams(c *fiber.Ctx) []string {
	raw := c.Request().URI().QueryArgs().PeekMulti("msgs")
	msgs := make([]string, len(raw))
	for i, b := range raw {
		msgs[i] = string(b)
	}
	return msgs
}

func GetParams(c *fiber.Ctx, key string) (string, error) {
	value := c.Params(key)
	if value == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, key+" param is required")
	}
	return value, nil
}

func GetParamInt(c *fiber.Ctx, key string) (int64, error) {
	value, err := GetParams(c, key)
	if err != nil {
		return 0, err
	}
	if value == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "missing parameter: "+key)
	}

	intValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid parameter: "+key+" - "+err.Error())
	}
	return intValue, nil
}

func GetAccountParam(c *fiber.Ctx) (sdk.AccAddress, error) {
	account, err := GetParams(c, "account")
	if err != nil {
		return nil, err
	}

	accAddr, err := util.AccAddressFromString(account)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid account: "+err.Error())
	}
	return accAddr, nil
}
