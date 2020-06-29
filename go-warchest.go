package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	nearapi "github.com/masknetgoal634/go-warchest/client"
	"github.com/masknetgoal634/go-warchest/common"
	"github.com/masknetgoal634/go-warchest/rpc"
	"github.com/masknetgoal634/go-warchest/runner"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "delegator ids"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var delegatorIds arrayFlags

func main() {
	log.Println("Go-Warchest started...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	url := flag.String("url", "https://rpc.betanet.near.org", "Near JSON-RPC URL")
	addr := flag.String("addr", ":9444", "listen address")
	poolId := flag.String("accountId", "test", "Validator pool account id")
	flag.Var(&delegatorIds, "delegatorId", "Delegator ids.")

	flag.Parse()
	if len(flag.Args()) > 0 {
		flag.Usage()
	}

	client := nearapi.NewClientWithContext(ctx, *url)

	// Prometheus metrics
	leftBlocksGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_left_blocks",
			Help: "The number of blocks left in the current epoch",
		})
	pingGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_ping",
			Help: "Near ping",
		})
	restakeGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_restake",
			Help: "Near stake/unstake event",
		})
	stakeAmountGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_stake_amount",
			Help: "The amount of stake",
		})
	nextSeatPriceGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_next_seat_price",
			Help: "The next seat price",
		})
	expectedSeatPriceGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_expected_seat_price",
			Help: "The expected seat price",
		})
	expectedStakeGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_expected_stake",
			Help: "The expected stake",
		})
	thresholdGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_threshold",
			Help: "The kickout threshold",
		})
	dStakedBalanceGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_delegator_staked_balance",
			Help: "The delegator staked balance",
		})
	dUnStakedBalanceGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_delegator_unstaked_balance",
			Help: "The delegator unstaked balance",
		})

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(leftBlocksGauge)
	registry.MustRegister(pingGauge)
	registry.MustRegister(restakeGauge)
	registry.MustRegister(stakeAmountGauge)
	registry.MustRegister(nextSeatPriceGauge)
	registry.MustRegister(expectedSeatPriceGauge)
	registry.MustRegister(expectedStakeGauge)
	registry.MustRegister(thresholdGauge)
	registry.MustRegister(dStakedBalanceGauge)
	registry.MustRegister(dUnStakedBalanceGauge)
	// Run a metrics service
	go runMetricsService(registry, *addr)

	monitor := rpc.NewMonitor(client, *poolId)
	resCh := make(chan *rpc.SubscrResult)
	// Quota for a concurrent rpc requests
	sem := make(common.Sem, 1)
	// Run a remote rpc monitor
	go monitor.Run(ctx, resCh, sem, thresholdGauge)

	runner := runner.NewRunner(*poolId, delegatorIds)
	// Run a near-shell runner
	runner.Run(ctx, resCh,
		leftBlocksGauge,
		pingGauge,
		restakeGauge,
		stakeAmountGauge,
		nextSeatPriceGauge,
		expectedSeatPriceGauge,
		expectedStakeGauge,
		dStakedBalanceGauge,
		dUnStakedBalanceGauge,
		sem)
}

func runMetricsService(registry prometheus.Gatherer, addr string) {
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:      log.New(os.Stderr, log.Prefix(), log.Flags()),
		ErrorHandling: promhttp.ContinueOnError,
	})
	http.Handle("/metrics", handler)
	log.Fatal(http.ListenAndServe(addr, nil))
}
