package common

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func GetParams(c *fiber.Ctx, key string) (string, error) {
	value := c.Params(key)
	if value == "" {
		return "", fmt.Errorf("missing parameter: %s", key)
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
		return 0, types.NewInvalidValueError("height", value, "must be a valid integer")
	}

	if intValue < 1 {
		return 0, types.NewInvalidValueError("height", fmt.Sprintf("%d", intValue), "must be a positive integer")
	}

	return intValue, nil
}

func GetAccountParam(c *fiber.Ctx) (string, error) {
	account, err := GetParams(c, "account")
	if err != nil {
		return "", err
	}

	accAddr, err := util.AccAddressFromString(account)
	if err != nil {
		return "", types.NewInvalidValueError("account", account, "invalid address format")
	}
	return accAddr.String(), nil
}

func GetCollectionAddrParam(c *fiber.Ctx, config *config.ChainConfig) ([]byte, error) {
	collectionAddr, err := GetParams(c, "collection_addr")
	if err != nil {
		return nil, err
	}

	return normalizeCollectionAddr(collectionAddr)
}

func GetMsgsQuery(c *fiber.Ctx) (msgs []string) {
	raw := c.Request().URI().QueryArgs().PeekMulti("msgs")
	for _, bytes := range raw {
		msgs = append(msgs, string(bytes))
	}
	return msgs
}

func GetCollectionAddrQuery(c *fiber.Ctx, config *config.ChainConfig) ([]byte, error) {
	collectionAddr := c.Query("collection_addr")
	if collectionAddr == "" {
		return nil, nil
	}

	return normalizeCollectionAddr(collectionAddr)
}
