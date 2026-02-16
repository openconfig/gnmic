package utils

import (
	"fmt"
	"strconv"
	"strings"
)

type RegisteredExtensions map[int32]string

func ParseRegisteredExtensions(pairs []string) (RegisteredExtensions, error) {
	res := RegisteredExtensions{}

	for _, p := range pairs {
		idMsg := strings.Split(p, ":")

		if len(idMsg) < 2 {
			return nil, fmt.Errorf("'%s' registered extension has invalid format, 123:package.Message format is expected", p)
		}

		id, err := strconv.ParseInt(idMsg[0], 10, 32)

		if err != nil {
			return nil, err
		}

		res[int32(id)] = idMsg[1]
	}

	return res, nil
}
