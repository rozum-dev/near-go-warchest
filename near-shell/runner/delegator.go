package runner

import (
	"fmt"

	"github.com/masknetgoal634/go-warchest/common"
	cmd "github.com/masknetgoal634/go-warchest/helpers"
)

func getDelegatorStakedBalance(poolId, delegatorId string) (int, error) {
	r, err := cmd.Run(fmt.Sprintf(getStakedBalanceCmd, poolId, delegatorId))
	if err != nil {
		return 0, err
	}
	return common.GetStakeFromNearView(r), nil
}

func getDelegatorUnStakedBalance(poolId, delegatorId string) (int, error) {
	r, err := cmd.Run(fmt.Sprintf(getUnStakedBalanceCmd, poolId, delegatorId))
	if err != nil {
		return 0, err
	}
	return common.GetStakeFromNearView(r), nil
}
