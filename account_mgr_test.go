package kn

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	//	corev1 "k8s.io/api/core/v1"
	// "encoding/base64"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func createK8S_ClientsetFromLocal() (*kubernetes.Clientset, error) {
	var kubeconfig *string
	path := filepath.Join(homeDir(), ".kube", "config")
	kubeconfig = &path

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	return clientset, nil
}

/*
func TestCreateServiceAccount(t *testing.T) {
	nameSpace := "knative-tests"
	saName := "sa-test"
	client, err := createK8S_ClientsetFromLocal()
	if err != nil {
		t.Error(err)
	}
	sa, err := client.CoreV1().ServiceAccounts(nameSpace).Create(
		context.TODO(),
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Namespace: nameSpace,
			},
		},
		metav1.CreateOptions{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ServiceAccount",
				APIVersion: "v1",
			},
		},
	)
	if err != nil {
		t.Error(err)
	}
	//fmt.Println(sa.Secrets[0])
}
*/

func TestGetSA(t *testing.T) {
	nameSpace := "default"
	saName := "knative-dev"
	client, err := createK8S_ClientsetFromLocal()
	if err != nil {
		t.Error(err)
	}
	sa, err := client.CoreV1().ServiceAccounts(nameSpace).Get(
		context.TODO(),
		saName,
		metav1.GetOptions{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ServiceAccount",
				APIVersion: "v1",
			},
		},
	)
	if err != nil {
		t.Error(err)
	}
	tokenName := sa.Secrets[0].Name
	fmt.Println(tokenName)
	scecret, err := client.CoreV1().Secrets(nameSpace).Get(
		context.TODO(),
		tokenName,
		metav1.GetOptions{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
		},
	)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(string(scecret.Data["token"]))
	//fmt.Println(base64.RawStdEncoding.EncodeToString(scecret.Data["token"]))

}
