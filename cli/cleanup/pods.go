package cleanup

import (
	"log"
	"time"

	"github.com/deliveroo/paddle/cli/pipeline"
	"github.com/spf13/cobra"
	v12 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var clientset kubernetes.Interface
var logFatalf = log.Fatalf

var podsCmd = &cobra.Command{
	Use:   "pods",
	Short: "Clean up all finished pods",
	Long: `Fetch all pods and delete the ones in complete state

Example:

$ paddle cleanup pods
`,
	Run: func(cmd *cobra.Command, args []string) {
		runPodsCleanup()
	},
}

func init() {
	config, err := pipeline.GetKubernetesConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

func runPodsCleanup() {
	pods := clientset.CoreV1().Pods("modeltraining")
	podList, err := pods.List(metav1.ListOptions{})
	if err != nil {
		logFatalf("[paddle] error fetching list of pods: %s", err.Error())
	}

	for _, pod := range podList.Items {
		switch string(pod.Status.Phase) {
		case "Succeeded":
			deletePod(pods, pod)
		case "Failed":
			diff := time.Now().UTC().Sub(pod.CreationTimestamp.UTC())
			if diff.Hours() >= 4 {
				deletePod(pods, pod)
			}
		default:
			continue
		}
	}
}

func deletePod(podInterface v1.PodInterface, pod v12.Pod) {
	err := podInterface.Delete(pod.Name, &metav1.DeleteOptions{})
	if err != nil {
		if k8errors.IsNotFound(err) {
			log.Printf("[paddle] deleted pod %s", pod.Name)
		} else {
			log.Printf("[paddle] error deleting pod %s", pod.Name)
		}
	}
	log.Printf("[paddle] deleted pod with pod name: %s, pod status: %s", pod.Name, string(pod.Status.Phase))
}
