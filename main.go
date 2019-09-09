package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stakater/helm-operator-metrics/pkg/collector"
	"github.com/stakater/helm-operator-metrics/pkg/options"

	log "github.com/sirupsen/logrus"
)

func healthcheck(collector *collector.HelmReleaseCollector) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Healthcheck has been called")
		if collector.IsHealthy() == true {
			w.Write([]byte("Ok"))
		} else {
			http.Error(w, "Healthcheck failed", http.StatusServiceUnavailable)
		}
	})
}

func main() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.JSONFormatter{})

	// Parse & validate environment variable
	opts := options.NewOptions()
	err := envconfig.Process("", opts)
	if err != nil {
		log.Fatal("Error parsing env vars into opts", err)
	}

	// Set log level from environment variable
	level, err := log.ParseLevel(opts.LogLevel)
	if err != nil {
		log.Panicf("Loglevel could not be parsed as one of the known loglevels. See logrus documentation for valid log level inputs. Given input was: '%s'", opts.LogLevel)
	}
	log.SetLevel(level)

	// Start helm operator metrics exporter
	log.Infof("Starting helm operator metrics exporter v%v", opts.Version)
	collector, err := collector.NewHelmReleaseCollector(opts)
	if err != nil {
		log.Fatalf("could not start helm operator metrics collector: '%v'", err)
	}
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/health", healthcheck(collector))
	address := fmt.Sprintf("%v:%s", opts.Host, strconv.Itoa(opts.Port))
	log.Info("Listening on ", address)
	log.Fatal(http.ListenAndServe(address, nil))

}
