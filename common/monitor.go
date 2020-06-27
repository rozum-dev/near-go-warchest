package common

import (
	"context"
	"fmt"
	"log"
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
	EpochLength       int
	CurrentStake      int
	NextStake         int
	KickedOut         bool
	Err               error
}

type Monitor struct {
	client    *nearapi.Client
	accountId string
	// cache
	result *SubscrResult
}

func NewMonitor(client *nearapi.Client, accountId string) *Monitor {
	return &Monitor{
		client:    client,
		accountId: accountId,
	}
}

func (m *Monitor) Run(ctx context.Context, result chan *SubscrResult, thresholdGauge prometheus.Gauge) {
	t := GetIntFromString(repeatTime)
	ticker := time.NewTicker(time.Duration(t) * time.Second)
	log.Printf("Subscribed for updates every %s seconds\n", repeatTime)
	for {
		select {
		case <-ticker.C:
			log.Println("Starting watch rpc")
			sr, err := m.client.Get("status", nil)
			if err != nil {
				fmt.Println(err)
				m.result.Err = err
				result <- m.result
				continue
			}

			var epochLength int
			switch sr.Status.ChainId {
			case "betanet":
				epochLength = 10000
			case "testnet":
				epochLength = 43200
			case "mainnet":
				epochLength = 43200
			}

			blockHeight := sr.Status.SyncInfo.LatestBlockHeight

			vr, err := m.client.Get("validators", []uint64{blockHeight})
			if err != nil {
				fmt.Println(err)
				m.result.Err = err
				result <- m.result
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

			m.result = &SubscrResult{
				LatestBlockHeight: int64(blockHeight),
				EpochStartHeight:  vr.Validators.EpochStartHeight,
				EpochLength:       epochLength,
				CurrentStake:      currentStake,
				NextStake:         nextStake,
				KickedOut:         kickedOut,
				Err:               nil,
			}

			result <- m.result

		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
