package runner

import "os"

var (
	currentSeatPriceCmd   = os.Getenv("CURRENT_SEAT_PRICE_CMD")
	nextSeatPriceCmd      = os.Getenv("NEXT_SEAT_PRICE_CMD")
	proposalsSeatPriceCmd = os.Getenv("PROPOSALS_SEAT_PRICE_CMD")
	proposalsCmd          = os.Getenv("PROPOSALS_CMD")

	stakeCmd              = os.Getenv("STAKE_CMD")
	getStakedBalanceCmd   = os.Getenv("GET_ACCOUNT_STAKED_BALANCE")
	getUnStakedBalanceCmd = os.Getenv("GET_ACCOUNT_UNSTAKED_BALANCE")

	pingCmd = os.Getenv("PING_CMD")
)
