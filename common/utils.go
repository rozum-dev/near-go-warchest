package common

import (
	"fmt"
	"strconv"
	"strings"
)

func GetStakeFromString(s string) int {
	if len(s) == 1 {
		return 0
	}
	l := len(s) - 19 - 5
	v, err := strconv.ParseFloat(s[0:l], 64)
	if err != nil {
		fmt.Println(err)
	}
	return int(v)
}

func GetIntFromString(s string) int {
	value := strings.Replace(s, ",", "", -1)
	value = strings.TrimSpace(value)
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return int(v)
}

func GetStringFromStake(stake int) string {
	return fmt.Sprintf("%d%s", stake, "0000000000000000000000000")
}
