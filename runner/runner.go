package runner

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/masknetgoal634/go-warchest/common"
	cmd "github.com/masknetgoal634/go-warchest/helpers"
	"github.com/prometheus/client_golang/prometheus"
)

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

type Runner struct {
	accountId                                          string
	delegatorId                                        string
	restaked                                           bool
	currentSeatPrice, nextSeatPrice, expectedSeatPrice int
	expectedStake                                      int
	rpcSuccess, rpcFailed                              int
}

func NewRunner(accountId, delegatorId string) *Runner {
	return &Runner{
		accountId:   accountId,
		delegatorId: delegatorId,
	}
}

func (r *Runner) Run(ctx context.Context, resCh chan *common.SubscrResult,
	leftBlocksGauge,
	pingGauge,
	restakeGauge,
	stakeAmountGauge,
	nextSeatPriceGauge,
	expectedSeatPriceGauge,
	expectedStakeGauge,
	dStakedBalanceGauge,
	dUnStakedBalanceGauge prometheus.Gauge) {

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	var epochStartHeight int64
	var delegatorStakedBalance, delegatorUnStakedBalance int
	var leftBlocksPrev, estimatedBlocksCountPerReq int // per 90 sec
	for {
		select {
		case res := <-resCh:
			if res.Err != nil {
				r.rpcFailed++
				log.Println("Failed to connect to RPC")
				if r.rpcSuccess > 0 {
					log.Println("Using cache...")
					res.LatestBlockHeight = res.LatestBlockHeight + int64(estimatedBlocksCountPerReq)
					// Estimated new epoch
					if res.LatestBlockHeight >= res.EpochStartHeight+int64(res.EpochLength) {
						res.EpochStartHeight += int64(res.EpochLength)
					}
				} else {
					continue
				}
			}
			r.rpcSuccess++
			if epochStartHeight == 0 {
				epochStartHeight = res.EpochStartHeight
				leftBlocksPrev = int(res.EpochStartHeight) - int(res.LatestBlockHeight) + res.EpochLength
			}
			leftBlocks := int(res.EpochStartHeight) - int(res.LatestBlockHeight) + res.EpochLength
			estimatedBlocksCountPerReq = leftBlocksPrev - leftBlocks
			leftBlocksPrev = leftBlocks
			log.Printf("LatestBlockHeight: %d\n", res.LatestBlockHeight)
			log.Printf("EpochStartHeight: %d\n", res.EpochStartHeight)
			log.Printf("Left Blocks: %d\n", leftBlocks)
			if res.KickedOut {
				continue
			}
			log.Printf("Current stake: %d\n", res.CurrentStake)
			log.Printf("Next stake: %d\n", res.NextStake)

			r.expectedStake = getExpectedStake(r.accountId)
			if r.expectedStake != 0 {
				log.Printf("Expected stake: %d\n", r.expectedStake)
				expectedStakeGauge.Set(float64(r.expectedStake))
			}
			dsb, err := getDelegatorStakedBalance(r.accountId, r.delegatorId)
			if err == nil {
				delegatorStakedBalance = dsb
				dStakedBalanceGauge.Set(float64(delegatorStakedBalance))
			}
			log.Printf("Delegator staked balance: %d\n", delegatorStakedBalance)

			dusb, err := getDelegatorUnStakedBalance(r.accountId, r.delegatorId)
			if err == nil {
				delegatorUnStakedBalance = dusb
				dUnStakedBalanceGauge.Set(float64(delegatorUnStakedBalance))
			}
			log.Printf("Delegator unstaked balance: %d\n", delegatorUnStakedBalance)

			leftBlocksGauge.Set(float64(leftBlocks))
			stakeAmountGauge.Set(float64(res.CurrentStake))
			restakeGauge.Set(0)
			pingGauge.Set(0)

			if epochStartHeight != res.EpochStartHeight {
				// New epoch
				// If the new epoch then ping
				log.Println("Starting ping...")
				command := fmt.Sprintf(pingCmd, r.accountId, r.delegatorId)
				_, err := cmd.Run(command)
				if err != nil {
					pingGauge.Set(0)
				} else {
					log.Printf("Success: %s\n", command)
					epochStartHeight = res.EpochStartHeight
					pingGauge.Set(float64(res.CurrentStake))
				}
			}

			if !r.fetchPrices(nextSeatPriceGauge, expectedSeatPriceGauge) {
				continue
			}
			// Seats calculation
			seats := float64(r.expectedStake) / float64(r.expectedSeatPrice)
			log.Printf("Expected seats: %f", seats)

			if seats >= 2.0 {
				log.Printf("You retain %f seats\n", seats)
				tokensAmount := getTokensAmountToRestake("unstake", r.expectedStake, delegatorStakedBalance, r.expectedSeatPrice)
				if leftBlocks < 1000 {
					// Run near unstake
					r.restake("unstake", tokensAmount, restakeGauge, stakeAmountGauge)
				} else {
					log.Printf("I will unstake %d later, there are still %d blocks left", tokensAmount, leftBlocks)
				}
			} else if seats < 1.0 {
				log.Printf("You don't have enough stake to get one seat: %f\n", seats)
				tokensAmount := getTokensAmountToRestake("stake", r.expectedStake, delegatorStakedBalance, r.expectedSeatPrice)
				if leftBlocks < 1000 {
					// Run near stake
					r.restake("stake", tokensAmount, restakeGauge, stakeAmountGauge)
				} else {
					log.Printf("I will stake %d later, there are still %d blocks left", tokensAmount, leftBlocks)
				}
			} else if seats >= 1.0 && seats < 2.0 {
				log.Println("I'm okay")
			}
		case <-ctx.Done():
			return
		case <-sigc:
			log.Println("System kill")
			os.Exit(0)
		}
	}
}

func (r *Runner) restake(method string, tokensAmount int, restakeGauge, stakeAmountGauge prometheus.Gauge) bool {
	if tokensAmount == 0 {
		return false
	}
	tokensAmountStr := common.GetStringFromStake(tokensAmount)
	stakeAmountGauge.Set(float64(tokensAmount))

	log.Printf("Starting %s %d...\n", method, tokensAmount)
	err := runStake(r.accountId, method, tokensAmountStr, r.delegatorId)
	if err != nil {
		return false
	}
	log.Printf("Success %sd %d NEAR\n", method, tokensAmount)
	restakeGauge.Set(float64(tokensAmount))

	return true
}

func runStake(poolId, method, amount, delegatorId string) error {
	_, err := cmd.Run(fmt.Sprintf(stakeCmd, poolId, method, amount, delegatorId))
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func getExpectedStake(accountId string) int {
	currentProp, err := cmd.Run(fmt.Sprintf(proposalsCmd, accountId))
	if err != nil {
		log.Printf("Failed to run proposalsCmd")
		return 0
	}
	if currentProp != "" {
		sa := strings.Split(currentProp, "|")
		if len(sa) >= 4 {
			s := sa[3]
			if len(strings.Fields(s)) > 1 {
				return common.GetIntFromString(strings.Fields(s)[2])
			} else {
				return common.GetIntFromString(strings.Fields(s)[0])
			}
		}
	}
	return 0
}

func (r *Runner) fetchPrices(nextSeatPriceGauge, expectedSeatPriceGauge prometheus.Gauge) bool {
	if r.currentSeatPrice == 0 {
		// Current seat price
		csp, err := getSeatPrice(currentSeatPriceCmd)
		if err != nil {
			log.Println("Failed to get currentSeatPrice")
			if r.currentSeatPrice == 0 {
				return false
			}
		} else {
			r.currentSeatPrice = csp
		}
		log.Printf("Current seat price %d\n", r.currentSeatPrice)
	}
	// Next seat price
	nsp, err := getSeatPrice(nextSeatPriceCmd)
	if err != nil {
		log.Println("Failed to get nextSeatPrice")
		if r.nextSeatPrice == 0 {
			return false
		}
	} else {
		r.nextSeatPrice = nsp
	}
	log.Printf("Next seat price %d\n", r.nextSeatPrice)
	nextSeatPriceGauge.Set(float64(r.nextSeatPrice))

	// Expected seat price
	esp, err := getSeatPrice(proposalsSeatPriceCmd)
	if err != nil {
		log.Println("Failed to get expectedSeatPrice")
		if r.expectedSeatPrice == 0 {
			return false
		}
	} else {
		r.expectedSeatPrice = esp
	}
	log.Printf("Expected seat price %d\n", r.expectedSeatPrice)
	expectedSeatPriceGauge.Set(float64(r.expectedSeatPrice))
	return true
}

func getSeatPrice(command string) (int, error) {
	r, err := cmd.Run(command)
	if err != nil {
		log.Printf("Failed to run %s", command)
		return 0, err
	}
	return common.GetIntFromString(r), nil
}

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

func getTokensAmountToRestake(method string, expectedStake, delegatorBalance, expectedSeatPrice int) int {
	var tokensAmount int
	if method == "stake" {
		tokensAmount = expectedSeatPrice - expectedStake + 1
		if tokensAmount > delegatorBalance {
			log.Printf("Not enough balance to stake %d NEAR\n", tokensAmount)
			return 0
		}
	} else {
		// unstake
		offset := 100
		for tokensAmount < delegatorBalance-offset && expectedStake-tokensAmount > expectedSeatPrice+offset {
			tokensAmount += offset
		}
	}
	return tokensAmount
}
