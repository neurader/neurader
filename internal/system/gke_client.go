package system

import (
	"fmt"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// GetGKEClient returns a clientset compatible with GKE environments
func GetGKEClient() *kubernetes.Clientset {
	var config *rest.Config
	var err error

	// 1. Try In-Cluster Config (Runs when NeuRader is a Pod in GKE)
	config, err = rest.InClusterConfig()
	if err != nil {
		// 2. Try Local Config (Runs when testing from your laptop)
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(fmt.Sprintf("[!] GKE Auth Error: Could not find cluster config: %v", err))
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return clientset
}
