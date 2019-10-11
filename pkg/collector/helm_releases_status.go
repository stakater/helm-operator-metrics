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
	statusConditionLabels := []string{"name", "namespace", "message", "reason", "status", "type", "transition_time", "update_time"}
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

		switch hr.Status.ReleaseStatus {
		case "UNKNOWN":
			releaseStatusValue = 0
		case "DEPLOYED":
			releaseStatusValue = 1
		case "DELETED":
			releaseStatusValue = 2
		case "SUPERSEDED":
			releaseStatusValue = 3
		case "FAILED":
			releaseStatusValue = -1
		case "DELETING":
			releaseStatusValue = 5
		case "PENDING_INSTALL":
			releaseStatusValue = 6
		case "PENDING_UPGRADE":
			releaseStatusValue = 7
		case "PENDING_ROLLBACK":
			releaseStatusValue = 8
		default: // invalid / non-existent status
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
			ch <- prometheus.MustNewConstMetric(c.helmReleaseStatusConditionDesc, prometheus.GaugeValue, conditionStatusValue, hr.Name, hr.Namespace, sc.Message, sc.Reason, string(sc.Status), string(sc.Type), sc.LastTransitionTime.String(), sc.LastUpdateTime.String())
		}
	}

	return nil

}
