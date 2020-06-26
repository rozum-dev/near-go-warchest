package common

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	nearapi "github.com/masknetgoal634/go-warchest/client"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	repeatTime = os.Getenv("REPEAT_TIME")
)

type SubscrResult struct {
	LatestBlockHeight int64
	EpochStartHeight  int64
	EpochLeight       int
	CurrentStake      int
	NextStake         int
	ExpectedStake     int
	KickedOut         bool
}

type Monitor struct {
	client    *nearapi.Client
	accountId string
}

func NewMonitor(client *nearapi.Client, accountId string) *Monitor {
	return &Monitor{
		client:    client,
		accountId: accountId,
	}
}

func (m *Monitor) Run(ctx context.Context, result chan *SubscrResult, thresholdGauge prometheus.Gauge) {
	t := rand.Int31n(int32(GetIntFromString(repeatTime)))
	ticker := time.NewTicker(time.Duration(t) * time.Second)
	log.Printf("Subscribed for updates every %s sec\n", repeatTime)
	for {
		select {
		case <-ticker.C:
			// Watch every ~180 sec
			log.Println("Starting watch rpc")
			sr, err := m.client.Get("status", nil)
			if err != nil {
				fmt.Println(err)
				continue
			}

			var epochLeight int
			switch sr.Status.ChainId {
			case "betanet":
				epochLeight = 10000
			case "testnet":
				epochLeight = 43200
			case "mainnet":
				epochLeight = 43200
			}

			blockHeight := sr.Status.SyncInfo.LatestBlockHeight

			vr, err := m.client.Get("validators", []uint64{blockHeight})
			if err != nil {
				fmt.Println(err)
				continue
			}

			kickedOut := true
			var currentStake int
			for _, v := range vr.Validators.CurrentValidators {
				if v.AccountId == m.accountId {
					pb := float64(v.NumProducedBlocks)
					eb := float64(v.NumExpectedBlocks)
					threshold := (pb / eb) * 100
					if threshold > 90.0 {
						log.Printf("Kicked out threshold: %f\n", threshold)
						kickedOut = false
					}
					thresholdGauge.Set(threshold)
					currentStake = GetStakeFromString(v.Stake)
				}
			}

			var nextStake int
			for _, v := range vr.Validators.NextValidators {
				if v.AccountId == m.accountId {
					nextStake = GetStakeFromString(v.Stake)
				}
			}

			var expectedStake int
			for _, v := range vr.Validators.CurrentProposals {
				if v.AccountId == m.accountId {
					expectedStake = GetStakeFromString(v.Stake)
				}
			}

			epochStartHeight := vr.Validators.EpochStartHeight

			r := &SubscrResult{
				LatestBlockHeight: int64(blockHeight),
				EpochStartHeight:  epochStartHeight,
				EpochLeight:       epochLeight,
				CurrentStake:      currentStake,
				NextStake:         nextStake,
				ExpectedStake:     expectedStake,
				KickedOut:         kickedOut,
			}

			result <- r

		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
