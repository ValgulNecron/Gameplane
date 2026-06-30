package controller

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// allGameServerPhases is the full set of phases a GameServer can report. The
// collector emits a sample for every phase on each scrape (0 when none are in
// that phase) so dashboards and alerts see a continuous series instead of a
// label that blinks in and out of existence as servers change state.
var allGameServerPhases = []gameplanev1alpha1.GameServerPhase{
	gameplanev1alpha1.GameServerPhasePending,
	gameplanev1alpha1.GameServerPhaseStarting,
	gameplanev1alpha1.GameServerPhaseRunning,
	gameplanev1alpha1.GameServerPhaseStopping,
	gameplanev1alpha1.GameServerPhaseStopped,
	gameplanev1alpha1.GameServerPhaseSuspended,
	gameplanev1alpha1.GameServerPhaseFailed,
}

var gameServersDesc = prometheus.NewDesc(
	"gameplane_gameservers",
	"Number of GameServers currently in each lifecycle phase, as observed by the operator.",
	[]string{"phase"}, nil,
)

// gameServerCollector reports fleet state by listing GameServers from the
// operator's shared informer cache at scrape time. Reading the cache is cheap
// and, unlike mutating a GaugeVec inside reconcile, sidesteps the stale-label
// and reset races you get when several reconciles run concurrently — the
// emitted series always reflects exactly what the cache holds right now.
type gameServerCollector struct {
	reader client.Reader
}

// NewGameServerCollector builds the fleet-metrics collector. Register it with
// the controller-runtime metrics registry so it is served on the operator's
// existing /metrics endpoint:
//
//	metrics.Registry.MustRegister(controller.NewGameServerCollector(mgr.GetClient()))
func NewGameServerCollector(reader client.Reader) prometheus.Collector {
	return &gameServerCollector{reader: reader}
}

func (c *gameServerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- gameServersDesc
}

func (c *gameServerCollector) Collect(ch chan<- prometheus.Metric) {
	var list gameplanev1alpha1.GameServerList
	if err := c.reader.List(context.Background(), &list); err != nil {
		// A scrape that lands mid-cache-sync (or a transient cache error)
		// must not fail the whole /metrics response; skip this collector's
		// samples and let the next scrape pick them up.
		ctrllog.Log.WithName("metrics").V(1).Info("gameserver fleet metric: list failed", "error", err)
		return
	}

	counts := make(map[gameplanev1alpha1.GameServerPhase]int, len(allGameServerPhases))
	for _, phase := range allGameServerPhases {
		counts[phase] = 0
	}
	for i := range list.Items {
		phase := list.Items[i].Status.Phase
		// A freshly-created GameServer the operator hasn't reconciled yet has
		// no phase; bucket it as Pending so the total still matches the fleet.
		if phase == "" {
			phase = gameplanev1alpha1.GameServerPhasePending
		}
		// Defensive: an unexpected phase value (none should exist) is ignored
		// rather than emitted as an unknown series.
		if _, known := counts[phase]; known {
			counts[phase]++
		}
	}

	for _, phase := range allGameServerPhases {
		ch <- prometheus.MustNewConstMetric(
			gameServersDesc, prometheus.GaugeValue, float64(counts[phase]), string(phase),
		)
	}
}
