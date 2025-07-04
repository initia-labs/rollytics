package common

import (
	"fmt"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/util"
)

func GetMsgsParams(c *fiber.Ctx) (msgs []string) {
	raw := c.Request().URI().QueryArgs().PeekMulti("msgs")
	for _, bytes := range raw {
		msgs = append(msgs, string(bytes))
	}
	return msgs
}

func GetParams(c *fiber.Ctx, key string) (string, error) {
	value := c.Params(key)
	if value == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("missing parameter: %s", key))
	}
	return value, nil
}

func GetHeightParam(c *fiber.Ctx) (int64, error) {
	value, err := GetParams(c, "height")
	if err != nil {
		return 0, err
	}

	intValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("invalid height: %s", err.Error()))
	}

	if intValue < 1 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "height must be positive integer")
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
		return nil, fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("invalid account: %s", err.Error()))
	}
	return accAddr, nil
}
