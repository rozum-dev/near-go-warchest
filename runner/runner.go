package runner

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/masknetgoal634/go-warchest/common"
	cmd "github.com/masknetgoal634/go-warchest/helpers"
	"github.com/masknetgoal634/go-warchest/rpc"
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
	poolId                                             string
	delegatorIds                                       []string
	defaultDelegatorId                                 string
	delegatorStakedBalance, delegatorUnStakedBalance   map[string]int
	restaked                                           bool
	currentSeatPrice, nextSeatPrice, expectedSeatPrice int
	expectedStake                                      int
	rpcSuccess, rpcFailed                              int
}

func NewRunner(poolId string, delegatorIds []string) *Runner {
	var defaultDelegatorId string
	delegatorStakedBalance := make(map[string]int)
	delegatorUnStakedBalance := make(map[string]int)
	for _, delegatorId := range delegatorIds {
		delegatorStakedBalance[delegatorId] = 0
		delegatorUnStakedBalance[delegatorId] = 0
		defaultDelegatorId = delegatorId
	}
	return &Runner{
		poolId:                   poolId,
		delegatorIds:             delegatorIds,
		defaultDelegatorId:       defaultDelegatorId,
		delegatorStakedBalance:   delegatorStakedBalance,
		delegatorUnStakedBalance: delegatorUnStakedBalance,
	}
}

func (r *Runner) Run(ctx context.Context, resCh chan *rpc.SubscrResult,
	leftBlocksGauge,
	pingGauge,
	restakeGauge,
	stakeAmountGauge,
	nextSeatPriceGauge,
	expectedSeatPriceGauge,
	expectedStakeGauge,
	dStakedBalanceGauge,
	dUnStakedBalanceGauge prometheus.Gauge,
	sem common.Sem) {

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	var notInProposals bool
	var epochStartHeight int64
	var leftBlocksPrev, estimatedBlocksCountPerReq int // per 90 sec
	for {
		select {
		case res := <-resCh:
			sem.Acquare()
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
					sem.Release()
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

			r.expectedStake = getExpectedStake(r.poolId)
			if r.expectedStake != 0 {
				log.Printf("Expected stake: %d\n", r.expectedStake)
				notInProposals = false
				expectedStakeGauge.Set(float64(r.expectedStake))
			} else {
				log.Printf("You are not in proposals\n")
				notInProposals = true
			}
			log.Printf("Current stake: %d\n", res.CurrentStake)
			log.Printf("Next stake: %d\n", res.NextStake)

			// multiple delegator accounts
			var totalDelegatorsStakedBalance, totalDelegatorsUnStakedBalance int
			for _, delegatorId := range r.delegatorIds {
				dsb, err := getDelegatorStakedBalance(r.poolId, delegatorId)
				if err == nil {
					r.delegatorStakedBalance[delegatorId] = dsb
					totalDelegatorsStakedBalance += dsb
				}
				log.Printf("%s staked balance: %d\n", delegatorId, dsb)

				dusb, err := getDelegatorUnStakedBalance(r.poolId, delegatorId)
				if err == nil {
					r.delegatorUnStakedBalance[delegatorId] = dusb
					totalDelegatorsUnStakedBalance += dusb
				}
				log.Printf("%s unstaked balance: %d\n", delegatorId, dusb)
			}
			dStakedBalanceGauge.Set(float64(totalDelegatorsStakedBalance))
			dUnStakedBalanceGauge.Set(float64(totalDelegatorsUnStakedBalance))

			leftBlocksGauge.Set(float64(leftBlocks))
			stakeAmountGauge.Set(float64(res.CurrentStake))
			restakeGauge.Set(0)
			pingGauge.Set(0)

			if epochStartHeight != res.EpochStartHeight {
				// New epoch
				// If the new epoch then ping
				log.Println("Starting ping...")
				command := fmt.Sprintf(pingCmd, r.poolId, r.defaultDelegatorId)
				_, err := cmd.Run(command)
				if err != nil {
					pingGauge.Set(0)
				} else {
					log.Printf("Success: %s\n", command)
					epochStartHeight = res.EpochStartHeight
					if res.CurrentStake == 0 {
						pingGauge.Set(float64(100000))
					} else {
						pingGauge.Set(float64(res.CurrentStake))
					}
				}
			}
			if !r.fetchPrices(nextSeatPriceGauge, expectedSeatPriceGauge) {
				sem.Release()
				continue
			}

			if notInProposals || res.KickedOut {
				sem.Release()
				continue
			}

			// Seats calculation
			seats := float64(r.expectedStake) / float64(r.expectedSeatPrice)
			log.Printf("Expected seats: %f", seats)

			if seats > 1.001 {
				log.Printf("You retain %f seats\n", seats)
				tokensAmountMap := getTokensAmountToRestake("unstake", r.delegatorStakedBalance, r.expectedStake, r.expectedSeatPrice)
				if len(tokensAmountMap) == 0 {
					log.Printf("You don't have enough staked balance\n")
					sem.Release()
					continue
				}
				// Run near unstake
				r.restake("unstake", tokensAmountMap, restakeGauge, stakeAmountGauge)
			} else if seats < 1.0 {
				log.Printf("You don't have enough stake to get one seat: %f\n", seats)
				tokensAmountMap := getTokensAmountToRestake("stake", r.delegatorUnStakedBalance, r.expectedStake, r.expectedSeatPrice)
				// Run near stake
				r.restake("stake", tokensAmountMap, restakeGauge, stakeAmountGauge)
			} else if seats >= 1.0 && seats < 1.001 {
				log.Println("I'm okay")
			}
			sem.Release()
		case <-ctx.Done():
			return
		case <-sigc:
			log.Println("System kill")
			os.Exit(0)
		}
	}
}

func (r *Runner) restake(method string, tokensAmountMap map[string]int, restakeGauge, stakeAmountGauge prometheus.Gauge) bool {
	if len(tokensAmountMap) == 0 {
		return false
	}
	for delegatorId, delegatorBalance := range tokensAmountMap {
		tokensAmountStr := common.GetStringFromStake(delegatorBalance)
		stakeAmountGauge.Set(float64(delegatorBalance))

		log.Printf("%s: Starting %s %d NEAR\n", delegatorId, method, delegatorBalance)
		err := runStake(r.poolId, method, tokensAmountStr, delegatorId)
		if err != nil {
			return false
		}
		log.Printf("%s: Success %sd %d NEAR\n", delegatorId, method, delegatorBalance)
		restakeGauge.Set(float64(delegatorBalance))
	}

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

type delegator struct {
	delegatorBalance int
	delegatorId      string
}

type entries []delegator

func (s entries) Len() int           { return len(s) }
func (s entries) Less(i, j int) bool { return s[i].delegatorBalance < s[j].delegatorBalance }
func (s entries) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func getTokensAmountToRestake(method string, delegatorBalances map[string]int, expectedStake, expectedSeatPrice int) map[string]int {
	var delegatorBalancesSorted entries
	for k, v := range delegatorBalances {
		delegatorBalancesSorted = append(delegatorBalancesSorted, delegator{delegatorBalance: v, delegatorId: k})
	}

	sort.Sort(sort.Reverse(delegatorBalancesSorted))

	tokensAmountMap := make(map[string]int)
	var balances []int
	for _, v := range delegatorBalancesSorted {
		var tokensAmount int
		// Stake
		if method == "stake" {
			tokensAmount = expectedSeatPrice - expectedStake + 100
			var sumOfStake int
			if len(balances) > 0 {
				for _, v := range balances {
					sumOfStake += v
				}
				sumOfStake += v.delegatorBalance
				if sumOfStake > tokensAmount {
					overage := sumOfStake - tokensAmount
					tokensAmountMap[v.delegatorId] = v.delegatorBalance - overage
					return tokensAmountMap
				}
			}

			if tokensAmount > v.delegatorBalance {
				log.Printf("%s not enough balance to stake %d NEAR\n", v.delegatorId, tokensAmount)
				tokensAmountMap[v.delegatorId] = v.delegatorBalance
				balances = append(balances, v.delegatorBalance)
				continue
			}
			tokensAmountMap[v.delegatorId] = tokensAmount
			return tokensAmountMap
		} else {
			// Unstake
			offset := 100
			for tokensAmount < v.delegatorBalance-offset && expectedStake-tokensAmount > expectedSeatPrice+offset {
				tokensAmount += offset
			}
			if tokensAmount == 0 {
				break
			}
			tokensAmountMap[v.delegatorId] = tokensAmount
			expectedStake -= tokensAmount
		}
	}
	return tokensAmountMap
}
