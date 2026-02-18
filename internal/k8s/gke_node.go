package k8s

import (
	"context"
	"fmt"
	"neurader/internal/cloud"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// StartNodeWatcher monitors the health of the GKE Nodes
func StartNodeWatcher(clientset *kubernetes.Clientset, uploader *cloud.GCSUploader) {
	fmt.Println("[*] Establishing Node Watcher...")

	for {
		watcher, err := clientset.CoreV1().Nodes().Watch(context.TODO(), metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			node, ok := event.Object.(*corev1.Node)
			if !ok {
				continue
			}

			// Check if Node is "NotReady"
			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" && condition.Status != "True" {
					fmt.Printf("[!] Node %s is UNHEALTHY (State: %s). Extracting System Logs...\n", node.Name, condition.Status)
					extractNodeSystemLogs(clientset, node.Name, uploader)
				}
			}
		}
	}
}

func extractNodeSystemLogs(clientset *kubernetes.Clientset, nodeName string, uploader *cloud.GCSUploader) {
	// GKE allows us to proxy into the node's log endpoint
	// This pulls the kubelet.log which tells us WHY the node died
	res := clientset.CoreV1().RESTClient().Get().
		Resource("nodes").
		Name(nodeName).
		SubResource("proxy").
		Suffix("logs/kubelet.log").
		Do(context.TODO())

	rawLogs, err := res.Raw()
	if err != nil {
		fmt.Printf("    [!] Could not reach Node Proxy for %s\n", nodeName)
		return
	}

	fileName := fmt.Sprintf("infrastructure/%s/kubelet-%d.log", nodeName, time.Now().Unix())
	uploader.Upload(fileName, rawLogs)
	fmt.Printf("    [+] Node Infrastructure logs archived: %s\n", fileName)
}
