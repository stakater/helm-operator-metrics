package collector

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/stakater/helm-operator-metrics/pkg/kubernetes"
	"github.com/stakater/helm-operator-metrics/pkg/options"
	helmreleasev1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	v1 "k8s.io/api/core/v1"
)

type helmReleaseStatusesCollector struct {
	kubernetesClient *kubernetes.Client

	helmReleaseStatusInfoDesc      *prometheus.Desc
	helmReleaseStatusConditionDesc *prometheus.Desc
}

func init() {
	registerCollector("status", helmReleaseStatusInfoCollector)
}

func helmReleaseStatusInfoCollector(opts *options.Options) (Collector, error) {
	subsystem := "status"
	statusInfoLabels := []string{"name", "namespace", "release_status", "release_name"}
	statusConditionLabels := []string{"name", "namespace", "message", "reason", "status", "type"}
	return &helmReleaseStatusesCollector{
		helmReleaseStatusInfoDesc: prometheus.NewDesc(prometheus.BuildFQName(opts.Namespace, subsystem, "info"),
			"HelmRelease Status information",
			statusInfoLabels,
			prometheus.Labels{}),
		helmReleaseStatusConditionDesc: prometheus.NewDesc(prometheus.BuildFQName(opts.Namespace, subsystem, "condition"),
			"HelmRelease Status Conditions",
			statusConditionLabels,
			prometheus.Labels{}),
	}, nil
}

func (c *helmReleaseStatusesCollector) updateMetrics(ch chan<- prometheus.Metric) error {
	log.Debug("Collecting helmrelease metrics")

	var wg sync.WaitGroup
	var helmreleaseList *helmreleasev1beta1.HelmReleaseList
	var helmreleaseListError error

	wg.Add(1)
	go func() {
		defer wg.Done()
		helmreleaseList, helmreleaseListError = kubernetesClient.HelmReleaseList()
	}()

	wg.Wait()

	if helmreleaseListError != nil {
		log.Warn("Failed to get helmreleaseList from Kubernetes", helmreleaseListError)
		return helmreleaseListError
	}

	for _, hr := range helmreleaseList.Items {
		var releaseStatusValue float64
		if hr.Status.ReleaseStatus == "DEPLOYED" {
			releaseStatusValue = 1
		} else {
			releaseStatusValue = -1
		}
		ch <- prometheus.MustNewConstMetric(c.helmReleaseStatusInfoDesc, prometheus.GaugeValue, releaseStatusValue, hr.Name, hr.Namespace, hr.Status.ReleaseStatus, hr.Status.ReleaseName)
		for _, sc := range hr.Status.Conditions {
			var conditionStatusValue float64
			if sc.Status == v1.ConditionTrue {
				conditionStatusValue = 1
			} else {
				conditionStatusValue = -1
			}
			ch <- prometheus.MustNewConstMetric(c.helmReleaseStatusConditionDesc, prometheus.GaugeValue, conditionStatusValue, hr.Name, hr.Namespace, sc.Message, sc.Reason, string(sc.Status), string(sc.Type))
		}
	}

	return nil

}
