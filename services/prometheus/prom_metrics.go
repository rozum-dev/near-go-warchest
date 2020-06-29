package prometheus

import (
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PromMetrics struct {
	LeftBlocksGauge        prometheus.Gauge
	PingGauge              prometheus.Gauge
	RestakeGauge           prometheus.Gauge
	StakeAmountGauge       prometheus.Gauge
	NextSeatPriceGauge     prometheus.Gauge
	ExpectedSeatPriceGauge prometheus.Gauge
	ExpectedStakeGauge     prometheus.Gauge
	ThresholdGauge         prometheus.Gauge
	DStakedBalanceGauge    prometheus.Gauge
	DUnStakedBalanceGauge  prometheus.Gauge
	registry               *prometheus.Registry
}

func NewPromMetrics() *PromMetrics {
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

	return &PromMetrics{
		LeftBlocksGauge:        leftBlocksGauge,
		PingGauge:              pingGauge,
		RestakeGauge:           restakeGauge,
		StakeAmountGauge:       stakeAmountGauge,
		NextSeatPriceGauge:     nextSeatPriceGauge,
		ExpectedSeatPriceGauge: expectedSeatPriceGauge,
		ExpectedStakeGauge:     expectedStakeGauge,
		ThresholdGauge:         thresholdGauge,
		DStakedBalanceGauge:    dStakedBalanceGauge,
		DUnStakedBalanceGauge:  dUnStakedBalanceGauge,
		registry:               registry,
	}
}

func (m *PromMetrics) RunMetricsService(addr string) {
	handler := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		ErrorLog:      log.New(os.Stderr, log.Prefix(), log.Flags()),
		ErrorHandling: promhttp.ContinueOnError,
	})
	http.Handle("/metrics", handler)
	log.Fatal(http.ListenAndServe(addr, nil))
}
