package pipeline

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func NewPodWatcher(c *kubernetes.Clientset, pod *v1.Pod, stopChannel chan bool) (<-chan watch.Event, error) {
	podSelector, err := fields.ParseSelector("metadata.name=" + pod.Name)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{
		FieldSelector: podSelector.String(),
		Watch:         true,
	}

	podWatch, err := c.CoreV1().Pods(pod.Namespace).Watch(options)

	eventCh := make(chan watch.Event, 30)

	go func() {
		defer podWatch.Stop()
		defer close(eventCh)
		var podWatchChannelClosed bool
		for {
			select {
			case _ = <-stopChannel:
				return

			case podEvent, ok := <-podWatch.ResultChan():
				if !ok {
					podWatchChannelClosed = true
				} else {
					eventCh <- podEvent
				}
			}
			if podWatchChannelClosed {
				break
			}
		}
	}()

	return eventCh, nil
}
