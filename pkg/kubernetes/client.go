package kubernetes

import (
	"fmt"
	"os"
	"path/filepath"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	log "github.com/sirupsen/logrus"
	"github.com/stakater/helm-operator-metrics/pkg/options"
	helmreleasev1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	helmrelease "github.com/weaveworks/flux/integrations/client/clientset/versioned/typed/flux.weave.works/v1beta1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client provides methods to get all required metrics from Kubernetes
type Client struct {
	apiClient         kubernetes.Interface
	helmReleaseClient helmrelease.FluxV1beta1Interface
}

// NewClient creates a new client to get data from kubernetes masters
func NewClient(opts *options.Options) (*Client, error) {
	// Get right config to connect to kubernetes
	var config *rest.Config
	if opts.IsInCluster {
		log.Info("Creating InCluster config to communicate with Kubernetes master")
		var err error
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		// Try to read currently set kubernetes config from your local kube config
		log.Info("Looking for Kubernetes config to communicate with Kubernetes master")
		kubeConfigPath, err := getKubeConfigPath()
		if err != nil {
			return nil, err
		}
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("read kubeconfig: %v", err)
		}
	}

	// We got two clients, one for the common API and one explicitly for metrics
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes main client: '%v'", err)
	}

	helmReleaseClient, err := helmrelease.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes helmrelease client: '%v'", err)
	}

	return &Client{
		apiClient:         client,
		helmReleaseClient: helmReleaseClient,
	}, nil
}

func (c *Client) HelmReleaseList() (*helmreleasev1beta1.HelmReleaseList, error) {
	list, err := c.helmReleaseClient.HelmReleases("").List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list, nil
}

// getKubeConfigPath returns the filepath to the local kubeConfig file or fails if it couldn't find it
func getKubeConfigPath() (string, error) {
	home := os.Getenv("HOME")

	// Mac OS
	if home != "" {
		configPath := filepath.Join(home, ".kube", "config")
		_, err := os.Stat(configPath)
		if err == nil {
			return configPath, nil
		}
	}

	// Windows
	home = os.Getenv("USERPROFILE")
	if home != "" {
		configPath := filepath.Join(home, ".kube", "config")
		_, err := os.Stat(configPath)
		if err == nil {
			return configPath, nil
		}
	}

	return "", fmt.Errorf("couldn't find home directory to look for the kube config")
}

// IsHealthy returns whether the kubernetes client is able to get a list of all pods
func (c *Client) IsHealthy() bool {
	_, err := c.HelmReleaseList()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("kubernetes client is not healthy")
		return false
	}

	return true
}
