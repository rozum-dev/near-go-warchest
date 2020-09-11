package common

import (
	"fmt"
	"strconv"
	"strings"
	"log"
)

func GetStakeFromString(s string) int {
	log.Print("Logging in Go!", s)
	if len(s) == 1 {
		return 0
	}
	l := len(s) - 19 - 5
	v, err := strconv.ParseFloat(s[0:l], 64)
	if err != nil {
		fmt.Println("fuck it")
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
	return fmt.Sprintf("%d%s", stake, "000000000000000000000000")
}

func GetStakeFromNearView(s string) int {
	s2 := strings.Split(s, "})")
	if len(s2) > 1 {
		s3 := s2[1]
		s4 := strings.Split(s3, "m")
		if len(s4) > 1 {
			s5 := strings.Replace(s4[1], "'", "", -1)
			s6 := s5[0 : len(s5)-4]
			return GetStakeFromString(s6)
		}
	}
	return 0
}
