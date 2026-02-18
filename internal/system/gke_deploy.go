package system

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployGKEResources sets up the necessary ServiceAccount and RBAC for NeuRader
func DeployGKEResources() {
	clientset := GetGKEClient()
	ns := "default"
	name := "neurader-gke-sa"

	fmt.Println("🛠️  Configuring GKE RBAC for NeuRader...")

	// 1. Create ServiceAccount
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := clientset.CoreV1().ServiceAccounts(ns).Create(context.TODO(), sa, metav1.CreateOptions{})
	if err == nil {
		fmt.Println("[+] Created ServiceAccount:", name)
	}

	// 2. Create ClusterRole (The "Eyes" to watch the cluster)
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "neurader-cluster-reader"},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/log", "nodes", "events"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes/proxy"},
				Verbs:     []string{"get"},
			},
		},
	}
	_, err = clientset.RbacV1().ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
	if err == nil {
		fmt.Println("[+] Created ClusterRole: neurader-cluster-reader")
	}

	// 3. Create Binding (Connects Account to Role)
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "neurader-global-binding"},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: name, Namespace: ns},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "neurader-cluster-reader",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	_, err = clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), binding, metav1.CreateOptions{})
	if err == nil {
		fmt.Println("[+] Created ClusterRoleBinding")
	}

	fmt.Println("✅ GKE Setup Complete. You can now run 'neurader gke-daemon'.")
}
