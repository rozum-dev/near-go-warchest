package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	nearapi "github.com/masknetgoal634/go-warchest/client"
	"github.com/masknetgoal634/go-warchest/common"
	"github.com/masknetgoal634/go-warchest/runner"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	log.Println("Go-Warchest started...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	url := flag.String("url", "https://rpc.betanet.near.org", "Near JSON-RPC URL")
	addr := flag.String("addr", ":9444", "listen address")
	accountId := flag.String("accountId", "test", "Validator pool account id")
	delegatorId := flag.String("delegatorId", "test", "Delegator account id")

	flag.Parse()
	if len(flag.Args()) > 0 {
		flag.Usage()
	}

	client := nearapi.NewClient(*url)

	// Prometheus metrics
	leftBlocksGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_left_blocks",
			Help: "Left Blocks",
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
			Help: "Stake amount",
		})
	nextSeatPriceGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_next_seat_price",
			Help: "The next seat price (updated when no more than 1000 blocks remain before the end of the epoch)",
		})
	thresholdGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "warchest_threshold",
			Help: "The kickout threshold",
		})

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(leftBlocksGauge)
	registry.MustRegister(pingGauge)
	registry.MustRegister(restakeGauge)
	registry.MustRegister(stakeAmountGauge)
	registry.MustRegister(nextSeatPriceGauge)
	registry.MustRegister(thresholdGauge)
	// Run a metrics service
	go runMetricsService(registry, *addr)

	monitor := common.NewMonitor(client, *accountId)
	resCh := make(chan *common.SubscrResult)
	// Run a remote rpc monitor
	go monitor.Run(ctx, resCh, thresholdGauge)

	runner := runner.NewRunner(*accountId, *delegatorId)
	// Run a near-shell runner
	runner.Run(ctx, resCh, leftBlocksGauge, pingGauge, restakeGauge, stakeAmountGauge, nextSeatPriceGauge)
}

func runMetricsService(registry prometheus.Gatherer, addr string) {
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:      log.New(os.Stderr, log.Prefix(), log.Flags()),
		ErrorHandling: promhttp.ContinueOnError,
	})
	http.Handle("/metrics", handler)
	log.Fatal(http.ListenAndServe(addr, nil))
}
