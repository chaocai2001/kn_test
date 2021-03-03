package kn_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	//	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/client/pkg/wait"

	//servingv1 "knative.dev/serving/pkg/v1"
	servinglib "knative.dev/client/pkg/serving"
	clientservingv1 "knative.dev/client/pkg/serving/v1"
	servingapiv1 "knative.dev/serving/pkg/apis/serving/v1"

	//"strings"
	//	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"

	//	"k8s.io/client-go/rest"
	ksv "knative.dev/client/pkg/serving/v1"
	servingv1client "knative.dev/serving/pkg/client/clientset/versioned/typed/serving/v1"
)

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func createClientsetFromLocal() (*rest.Config, error) {
	var kubeconfig *string
	path := filepath.Join(homeDir(), ".kube", "config")
	kubeconfig = &path

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)

	return config, err
}

func getKNativeClient(namespace string) (clientservingv1.KnServingClient, error) {
	restConfig, err := createClientsetFromLocal()
	if err != nil {
		return nil, err
	}
	client, err := servingv1client.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	ksc := ksv.NewKnServingClient(client, namespace)
	return ksc, nil
}

func TestListKSVC(t *testing.T) {
	ksc, err := getKNativeClient("knative-tests")
	if err != nil {
		t.Error(err)
	}
	list, err := ksc.ListRoutes()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(list)
}

func constructServiceFromFile(fileName string) (*servingapiv1.Service, error) {
	var service servingapiv1.Service
	file, err := os.Open("service.yaml")
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewYAMLOrJSONDecoder(file, 512)

	err = decoder.Decode(&service)
	if err != nil {
		return nil, err
	}
	return &service, nil
}

// 1 Lauch a new service, (including replica set, service, virtual service, ingress, ...)
func TestCreateService(t *testing.T) {
	service, err := constructServiceFromFile("service.yaml")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("service_structure:", *service)
	ksc, err := getKNativeClient(service.GetNamespace())
	if err != nil {
		t.Error(err)
	}

	// if _, err = ksc.GetService(service.GetName()); err == nil {

	// 	err = ksc.DeleteService(service.GetName(), time.Second*30)
	// 	fmt.Println("delete error:", err)

	// 	//time.Sleep(time.Second * 5)
	// 	fmt.Println("Cleared env.")
	// }
	err = ksc.CreateService(service)

	if err != nil {
		t.Error(err)
	}
	buf := new(bytes.Buffer)
	ksc.WaitForService(service.GetName(), time.Second*10, wait.SimpleMessageCallback(buf))
	fmt.Println("wait callback:", buf.String())
	runningService, err := ksc.GetService(service.GetName())
	if err != nil {
		t.Errorf("cannot fetch service '%s' in namespace '%s' for extracting the URL: %v", service.GetName(), ksc.Namespace(), err)
	}

	url := runningService.Status.URL.String()

	newRevision := runningService.Status.LatestCreatedRevisionName

	fmt.Printf("Service '%s' to latest revision '%s' is available at URL:\n%s\n", service.GetName(), newRevision, url)

}

func CreateNewRevision(serviceName string, ksc clientservingv1.KnServingClient) error {
	service, err := ksc.GetService(serviceName)
	if err != nil {
		return err
	}
	//servinglib.GenerateRevisionName(""{{.Service}}-{{.Random 5}}-{{.Generation}}"")
	revName, err := servinglib.GenerateRevisionName("{{.Service}}-{{.Generation}}", service)
	if err != nil {
		return err
	}

	updateFn := func(service *servingapiv1.Service) (*servingapiv1.Service, error) {
		// update service for the new revision
		service.Spec.Template.Spec.PodSpec.Containers[0].Env = []corev1.EnvVar{
			corev1.EnvVar{"TARGET", "Sample v2", nil},
		}

		service.Spec.Traffic = []servingapiv1.TrafficTarget{}

		// lauch the new revision
		service.Spec.Template.Name = revName
		return service, nil
	}
	err = ksc.UpdateServiceWithRetry(service.GetName(), updateFn, 5)

	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	ksc.WaitForService(service.GetName(), time.Second*10, wait.SimpleMessageCallback(buf))
	fmt.Println("wait callback:", buf.String())
	runningService, err := ksc.GetService(service.GetName())
	if err != nil {
		return err
	}

	url := runningService.Status.URL.String()

	newRevision := runningService.Status.LatestCreatedRevisionName

	fmt.Printf("Service '%s' to latest revision '%s' is available at URL:\n%s\n", service.GetName(), newRevision, url)
	return nil
}

func PtrBool(v bool) *bool {
	return &v
}

// 2 Create a new revision of existing service
// 3 Canary launching
func TestCanaryLaunch(t *testing.T) {
	ksc, err := getKNativeClient("knative-tests")
	if err != nil {
		t.Error(err)
	}
	err = CreateNewRevision("hello-kn-client", ksc)
	if err != nil {
		t.Error(err)
	}
	service, err := ksc.GetService("hello-kn-client")
	if err != nil {
		t.Error(err)
	}
	//servinglib.GenerateRevisionName(""{{.Service}}-{{.Random 5}}-{{.Generation}}"")
	//	revName, err := servinglib.GenerateRevisionName("{{.Service}}-{{.Generation}}", service)
	if err != nil {
		t.Error(err)
	}

	updateFn := func(service *servingapiv1.Service) (*servingapiv1.Service, error) {
		canaryRate := int64(50)
		orgRate := 100 - canaryRate

		service.Spec.Traffic = []servingapiv1.TrafficTarget{
			servingapiv1.TrafficTarget{
				LatestRevision: PtrBool(false),
				RevisionName:   "hello-kn-client-00001",
				Percent:        &orgRate,
			},
			servingapiv1.TrafficTarget{
				LatestRevision: PtrBool(true),
				//RevisionName: "hello-kn-client-13",
				Percent: &canaryRate,
			},
		}
		return service, nil
	}
	err = ksc.UpdateServiceWithRetry(service.GetName(), updateFn, 5)
	if err != nil {
		t.Error(err)
	}

	buf := new(bytes.Buffer)
	ksc.WaitForService(service.GetName(), time.Second*10, wait.SimpleMessageCallback(buf))
	fmt.Println("wait callback:", buf.String())
	runningService, err := ksc.GetService(service.GetName())
	if err != nil {
		t.Errorf("cannot fetch service '%s' in namespace '%s' for extracting the URL: %v", service.GetName(), ksc.Namespace(), err)
	}

	url := runningService.Status.URL.String()

	newRevision := runningService.Status.LatestCreatedRevisionName

	fmt.Printf("Service '%s' to latest revision '%s' is available at URL:\n%s\n", service.GetName(), newRevision, url)

}
