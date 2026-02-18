package k8s

import (
	"context"
	"fmt"
	"io"
	"neurader/internal/cloud"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// StartWatcher initiates a 24/7 watch on the K8s API for pod failures
func StartWatcher(clientset *kubernetes.Clientset, uploader *cloud.GCSUploader) {
	fmt.Println("[*] Establishing Watcher on all namespaces...")

	for {
		// Use a high-level Watcher to monitor pod events
		watcher, err := clientset.CoreV1().Pods("").Watch(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("[!] Watch error: %v. Retrying in 5s...\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			// Check if any container in the pod is failing
			for _, status := range pod.Status.ContainerStatuses {
				if status.State.Waiting != nil && (status.State.Waiting.Reason == "CrashLoopBackOff" || status.State.Waiting.Reason == "Error") {
					fmt.Printf("[!] Failure detected in %s/%s. Extracting logs...\n", pod.Namespace, pod.Name)
					
					// Sniper Extract: Grab current and previous logs
					extractAndUploadLogs(clientset, pod, status.Name, uploader)
				}
			}
		}
		fmt.Println("[*] Watcher channel closed. Restarting...")
	}
}

func extractAndUploadLogs(clientset *kubernetes.Clientset, pod *corev1.Pod, containerName string, uploader *cloud.GCSUploader) {
	// Log options to get the "Death Note" (the logs from the crash)
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		Previous:  true, // CRITICAL: Gets the logs that caused the crash
	}

	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOptions)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		fmt.Printf("    [?] No previous logs found for %s. Trying current logs...\n", containerName)
		logOptions.Previous = false
		req = clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOptions)
		podLogs, err = req.Stream(context.TODO())
		if err != nil {
			return
		}
	}
	defer podLogs.Close()

	// Convert stream to string/bytes and upload
	data, _ := io.ReadAll(podLogs)
	fileName := fmt.Sprintf("%s/%s-%d.log", pod.Namespace, pod.Name, time.Now().Unix())
	
	err = uploader.Upload(fileName, data)
	if err != nil {
		fmt.Printf("    [!] Upload failed: %v\n", err)
	} else {
		fmt.Printf("    [+] Logs archived to GCS: %s\n", fileName)
	}
}
