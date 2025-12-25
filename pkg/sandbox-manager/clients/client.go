package clients

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	sandboxclient "github.com/openkruise/agents/client/clientset/versioned"
	sandboxfake "github.com/openkruise/agents/client/clientset/versioned/fake"
	"github.com/openkruise/agents/pkg/sandbox-manager/consts"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

type K8sClient kubernetes.Interface
type SandboxClient sandboxclient.Interface

type ClientSet struct {
	K8sClient
	SandboxClient
	*rest.Config
}

func NewClientSet(infra string) (*ClientSet, error) {
	client := &ClientSet{}
	// Try to use in-cluster config first (when running inside a Kubernetes pod)
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig file if not running in cluster
		var kubeconfig string

		// Check if kubeconfig is set in environment variable
		if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
			kubeconfig = kubeconfigEnv
		} else {
			// Use default kubeconfig path
			if home := homedir.HomeDir(); home != "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}

		// Use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	}

	// Configure rate limiter to handle client-side throttling
	// These values can be adjusted based on your cluster's capacity and requirements
	// Default values are typically QPS=5, Burst=10 which might be too low for active applications
	// QPS (Queries Per Second): Maximum requests per second to the API server
	// Burst: Maximum burst requests allowed in a short period
	// For high-activity applications, increasing these can reduce client-side throttling
	// Be careful not to set these too high as it might overload the Kubernetes API server
	// These can be configured via environment variables:
	// KUBE_CLIENT_QPS (default: 500)
	// KUBE_CLIENT_BURST (default: 1000)
	config.QPS = 500    // Default QPS
	config.Burst = 1000 // Default Burst

	// Override with environment variables if set
	if qpsStr := os.Getenv("KUBE_CLIENT_QPS"); qpsStr != "" {
		if qps, err := strconv.ParseFloat(qpsStr, 32); err == nil {
			config.QPS = float32(qps)
		}
	}
	if burstStr := os.Getenv("KUBE_CLIENT_BURST"); burstStr != "" {
		if burst, err := strconv.Atoi(burstStr); err == nil {
			config.Burst = burst
		}
	}
	client.Config = config
	klog.InfoS("client config", "qps", config.QPS, "burst", config.Burst)
	// Create the client
	client.K8sClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	if infra == consts.InfraSandboxCR {
		client.SandboxClient, err = sandboxclient.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create sandbox client: %w", err)
		}
	}

	return client, nil
}

//goland:noinspection GoDeprecation
func NewFakeClientSet() *ClientSet {
	client := &ClientSet{}
	client.K8sClient = k8sfake.NewClientset()

	// Create fake sandbox client
	fakeSandboxClient := sandboxfake.NewSimpleClientset()
	client.SandboxClient = fakeSandboxClient

	// Add reactor to auto-increment ResourceVersion for fake client
	// This simulates real Kubernetes API server behavior in tests
	AddResourceVersionReactor(fakeSandboxClient)

	return client
}

// AddResourceVersionReactor adds a reactor to fake client that auto-increments ResourceVersion
// on create/update operations, simulating real Kubernetes API server behavior.
//
// Background:
// - Real K8s API server automatically increments ResourceVersion on every update
// - Fake client doesn't do this by default, causing issues with cache keys that include RV
// - This reactor intercepts create/update actions and ensures RV is incremented
//
// Usage:
//
//	client := sandboxfake.NewSimpleClientset()
//	AddResourceVersionReactor(client)
func AddResourceVersionReactor(fakeClient *sandboxfake.Clientset) {
	// Counter for generating unique ResourceVersions
	// Start from 1 to match test helper functions like AvoidGetFromCache (which uses "100")
	var resourceVersionCounter int64 = 1

	// PrependReactor adds a reactor that runs BEFORE the default fake client logic
	// This allows us to intercept and modify objects before they're stored
	fakeClient.PrependReactor("create", "*", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
		createAction, ok := action.(testing.CreateAction)
		if !ok {
			return false, nil, nil
		}

		obj := createAction.GetObject()
		if obj == nil {
			return false, nil, nil
		}

		// Get metav1.Object interface to access/modify metadata
		metaObj, err := meta.Accessor(obj)
		if err != nil {
			return false, nil, err
		}

		// If ResourceVersion is empty or "0", assign initial version
		rv := metaObj.GetResourceVersion()
		if rv == "" || rv == "0" {
			newRV := atomic.AddInt64(&resourceVersionCounter, 1)
			metaObj.SetResourceVersion(strconv.FormatInt(newRV, 10))
		}

		// Return false to continue with default fake client logic
		return false, nil, nil
	})

	fakeClient.PrependReactor("update", "*", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
		updateAction, ok := action.(testing.UpdateAction)
		if !ok {
			return false, nil, nil
		}

		obj := updateAction.GetObject()
		if obj == nil {
			return false, nil, nil
		}

		// Get metav1.Object interface
		metaObj, err := meta.Accessor(obj)
		if err != nil {
			return false, nil, err
		}

		// Always increment ResourceVersion on update
		newRV := atomic.AddInt64(&resourceVersionCounter, 1)
		metaObj.SetResourceVersion(strconv.FormatInt(newRV, 10))

		// Return false to continue with default fake client logic
		return false, nil, nil
	})
}

// GetCurrentResourceVersion returns the current resource version counter value
// Useful for debugging in tests
func GetCurrentResourceVersion() int64 {
	// Note: This requires the counter to be accessible, which would need refactoring
	// For now, this is just a placeholder
	return 0
}
