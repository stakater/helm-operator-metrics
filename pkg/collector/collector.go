package collector

import (
	"fmt"
	"sync"
	"time"

	"github.com/stakater/helm-operator-metrics/pkg/kubernetes"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/stakater/helm-operator-metrics/pkg/options"
)

type collectorFactoryFunc = func(opts *options.Options) (Collector, error)

var (
	kubernetesClient         *kubernetes.Client
	scrapeDurationDesc       *prometheus.Desc
	scrapeSuccessDesc        *prometheus.Desc
	factoriesByCollectorName = make(map[string]collectorFactoryFunc)
)

// registerCollector adds a collector to the registry so that it's Update() method will be called every time
// the metrics endpoint is triggered
func registerCollector(collectorName string, collectorFactory collectorFactoryFunc) {
	log.Debugf("Registering collector '%s'", collectorName)
	factoriesByCollectorName[collectorName] = collectorFactory
}

// HelmReleaseCollector implements the prometheus collector interface
type HelmReleaseCollector struct {
	CollectorByName map[string]Collector
}

// NewHelmReleaseCollector creates a new HelmRelease collector which can be considered as manager of multiple collectors
func NewHelmReleaseCollector(opts *options.Options) (*HelmReleaseCollector, error) {
	// Create registered collectors by executing it's collector factory function
	collectorByName := make(map[string]Collector)
	for collectorName, factory := range factoriesByCollectorName {
		log.Debugf("Creating collector '%s'", collectorName)
		collector, err := factory(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to create collector '%s': '%s'", collectorName, err)
		}
		collectorByName[collectorName] = collector
	}

	var err error
	kubernetesClient, err = kubernetes.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kubernetes client: '%v'", err)
	}

	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(opts.Namespace, "scrape", "collector_duration_seconds"),
		"HelmRelease: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(opts.Namespace, "scrape", "collector_success"),
		"HelmRelease: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)

	return &HelmReleaseCollector{CollectorByName: collectorByName}, nil
}

// Describe implements the prometheus.Collector interface
func (k HelmReleaseCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface
func (k HelmReleaseCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}

	// Run all collectors concurrently and add meta information about that (such as request duration and error/success count)
	for name, collector := range k.CollectorByName {
		wg.Add(1)
		go func(wg *sync.WaitGroup, collectorName string, c Collector) {
			defer wg.Done()
			begin := time.Now()
			err := c.updateMetrics(ch)
			duration := time.Since(begin)

			var isSuccess float64
			if err != nil {
				log.Errorf("Collector '%s' failed after %fs: %s", collectorName, duration.Seconds(), err)
				isSuccess = 0
			} else {
				log.Debugf("Collector '%s' succeeded after  %fs.", collectorName, duration.Seconds())
				isSuccess = 1
			}
			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), collectorName)
			ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, isSuccess, collectorName)
		}(&wg, name, collector)
	}
	wg.Wait()
}

// IsHealthy returns a bool which indicates whether the collector is working properly or not
func (k HelmReleaseCollector) IsHealthy() bool {
	return kubernetesClient.IsHealthy()
}

// Collector is an interface which has to be implemented for each collector which wants to expose metrics
type Collector interface {
	updateMetrics(ch chan<- prometheus.Metric) error
}
