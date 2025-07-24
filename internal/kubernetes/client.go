package kubernetes

import (
	"context"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/kevinfinalboss/privateer/internal/config"
	"github.com/kevinfinalboss/privateer/internal/logger"
)

type Client struct {
	clientset *kubernetes.Clientset
	config    *config.Config
	logger    *logger.Logger
}

func NewClient(cfg *config.Config, log *logger.Logger) (*Client, error) {
	log.Info("connecting_k8s").Send()

	kubeconfig := getKubeconfigPath()

	configLoader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{CurrentContext: cfg.Kubernetes.Context},
	)

	restConfig, err := configLoader.ClientConfig()
	if err != nil {
		log.Error("k8s_connection_failed").Err(err).Send()
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Error("k8s_connection_failed").Err(err).Send()
		return nil, err
	}

	client := &Client{
		clientset: clientset,
		config:    cfg,
		logger:    log,
	}

	if err := client.validateConnection(); err != nil {
		log.Error("k8s_connection_failed").Err(err).Send()
		return nil, err
	}

	currentContext, _ := configLoader.RawConfig()
	contextName := currentContext.CurrentContext
	if cfg.Kubernetes.Context != "" {
		contextName = cfg.Kubernetes.Context
	}

	log.Info("k8s_connected").Str("context", contextName).Send()

	return client, nil
}

func getKubeconfigPath() string {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}

	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}

	return ""
}

func (c *Client) validateConnection() error {
	ctx := context.Background()

	_, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	return err
}

func (c *Client) GetNamespaces() ([]string, error) {
	ctx := context.Background()

	if len(c.config.Kubernetes.Namespaces) > 0 {
		c.logger.Debug("using_configured_namespaces").
			Strs("namespaces", c.config.Kubernetes.Namespaces).
			Send()
		return c.config.Kubernetes.Namespaces, nil
	}

	namespaceList, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var namespaces []string
	for _, ns := range namespaceList.Items {
		namespaces = append(namespaces, ns.Name)
	}

	c.logger.Debug("discovered_namespaces").
		Int("count", len(namespaces)).
		Strs("namespaces", namespaces).
		Send()

	return namespaces, nil
}

func (c *Client) GetClient() *kubernetes.Clientset {
	return c.clientset
}
