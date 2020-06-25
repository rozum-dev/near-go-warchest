package runner

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/masknetgoal634/go-warchest/common"
	cmd "github.com/masknetgoal634/go-warchest/helpers"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	currentSeatPriceCmd   = os.Getenv("CURRENT_SEAT_PRICE_CMD")
	nextSeatPriceCmd      = os.Getenv("NEXT_SEAT_PRICE_CMD")
	proposalsSeatPriceCmd = os.Getenv("PROPOSALS_SEAT_PRICE_CMD")

	stakeCmd     = os.Getenv("STAKE_CMD")
	proposalsCmd = os.Getenv("PROPOSALS_CMD")

	pingCmd = os.Getenv("PING_CMD")
)

type Runner struct {
	loggerProm   *prometheus.GaugeVec
	accountId    string
	delegatorId  string
	epochsNeeded int
	restaked     bool
}

func NewRunner(accountId, delegatorId string) *Runner {
	return &Runner{
		accountId:    accountId,
		delegatorId:  delegatorId,
		epochsNeeded: 3,
	}
}

func (runner *Runner) Run(ctx context.Context, resCh chan *common.SubscrResult, leftBlocksGauge, pingGauge, restakeGauge, stakeAmountGauge, nextSeatPriceGauge prometheus.Gauge) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	var imOk bool
	var epochStartHeight int64
	var currentSeatPrice, nextSeatPrice int //, proposalsSeatPrice int

	for {
		select {
		case r := <-resCh:
			if epochStartHeight == 0 {
				epochStartHeight = r.EpochStartHeight
			}
			leftBlocks := int(r.EpochStartHeight) - int(r.LatestBlockHeight) + r.EpochLeight
			log.Printf("LatestBlockHeight: %d\n", r.LatestBlockHeight)
			log.Printf("EpochStartHeight: %d\n", r.EpochStartHeight)
			log.Printf("Left Blocks %d\n", leftBlocks)

			leftBlocksGauge.Set(float64(leftBlocks))
			stakeAmountGauge.Set(float64(r.CurrentStake))

			if epochStartHeight != r.EpochStartHeight {
				// New epoch
				imOk = false
				if runner.epochsNeeded > 0 && runner.restaked {
					runner.epochsNeeded--
				} else {
					runner.epochsNeeded = 3
					runner.restaked = false
				}
				// If the new epoch then ping
				log.Println("Starting ping...")
				command := fmt.Sprintf(pingCmd, runner.accountId, runner.delegatorId)
				_, err := cmd.Run(command)
				log.Println(command)
				if err != nil {
					epochStartHeight = r.EpochStartHeight
				}
				pingGauge.Set(1.0)
			}
			if leftBlocks <= 1000 && !imOk {
				// Get next seat price
				if currentSeatPrice == 0 {
					csp, err := getSeatPrice(currentSeatPriceCmd)
					if err != nil {
						log.Println("Failed to get currentSeatPrice")
						if currentSeatPrice == 0 {
							continue
						}
					} else {
						currentSeatPrice = csp
					}
					log.Printf("Current seat price %d\n", currentSeatPrice)
				}
				nsp, err := getSeatPrice(nextSeatPriceCmd)
				if err != nil {
					log.Println("Failed to get nextSeatPriceCmd")
					if nextSeatPrice == 0 {
						continue
					}
				} else {
					nextSeatPrice = nsp
				}
				nextSeatPriceGauge.Set(float64(nextSeatPrice))

				seats := float64(r.CurrentStake) / float64(nextSeatPrice)
				log.Printf("I have seats: %f", seats)

				log.Println("Starting near proposals cmd...")
				prewProp, err := cmd.Run(fmt.Sprintf(proposalsCmd, runner.accountId))
				if err != nil {
					log.Println("Failed to run proposalsCmd")
					prewProp = ""
				}
				offset := 500
				if seats >= 2.0 && runner.epochsNeeded == 3 {
					log.Printf("You retain two or more seats: %f\n", seats)
					// Run near unstake
					runner.restake("unstake", prewProp, r.CurrentStake, nextSeatPrice, offset, restakeGauge, stakeAmountGauge)
				} else if seats < 1 && runner.epochsNeeded == 3 {
					log.Printf("You don't have enough stake to get one seat: %f\n", seats)
					// Run near stake
					runner.restake("stake", prewProp, r.CurrentStake, nextSeatPrice, offset, restakeGauge, stakeAmountGauge)
				} else if seats >= 1.0 && seats < 2.0 {
					imOk = true
				}
			}
		case <-ctx.Done():
			return
		case <-sigc:
			log.Println("System kill")
			os.Exit(0)
		}
	}
}

func (runner *Runner) restake(method, prewProp string, currentStake, nextSeatPrice, offset int, restakeGauge, stakeAmountGauge prometheus.Gauge) bool {
	var newStakeStr string
	var newStake int
	if method == "stake" {
		newStake = nextSeatPrice - currentStake + offset
		newStakeStr = common.GetStringFromStake(newStake)
	} else {
		// unstake
		newStake := currentStake - nextSeatPrice + offset
		newStakeStr = common.GetStringFromStake(newStake)
	}
	stakeAmountGauge.Set(float64(newStake))

	log.Printf("Starting %s...\n", method)
	err2 := runStake(runner.accountId, method, newStakeStr, runner.delegatorId)
	if err2 != nil {
		return false
	}
	restakeGauge.Set(1.0)

	currentProp, err := cmd.Run(fmt.Sprintf(proposalsCmd, runner.accountId))
	if err != nil {
		log.Printf("Failed to run proposalsCmd")
		return false
	}
	if currentProp != prewProp {
		runner.restaked = true
		runner.epochsNeeded--
	}
	return true
}

func runStake(poolId, method, amount, delegatorId string) error {
	command := fmt.Sprintf(stakeCmd, poolId, method, amount, delegatorId)
	_, err := cmd.Run(command)
	if err != nil {
		log.Printf("Failed to run %s", command)
		return err
	}
	return nil
}

func getSeatPrice(command string) (int, error) {
	r, err := cmd.Run(command)
	if err != nil {
		log.Printf("Failed to run %s", command)
		return 0, err
	}
	return common.GetIntFromString(r), nil
}
