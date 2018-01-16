package pipeline

import (
	"bufio"
	"context"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"log"
	"time"
)

type WatchEventType string

const (
	Added     WatchEventType = "ADDED"
	Deleted   WatchEventType = "DELETED"
	Removed   WatchEventType = "REMOVED"
	Completed WatchEventType = "COMPLETED"
	Failed    WatchEventType = "FAILED"
)

const watchPollInterval = 30 * time.Second

type WatchEvent struct {
	Type      WatchEventType
	Pod       *v1.Pod
	Container string
	Message   string
}

func Watch(ctx context.Context, c kubernetes.Interface, watchPod *v1.Pod) (<-chan WatchEvent, error) {
	podSelector, err := fields.ParseSelector("metadata.name=" + watchPod.Name)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{
		FieldSelector: podSelector.String(),
		Watch:         true,
	}

	watcher, err := c.CoreV1().Pods(watchPod.Namespace).Watch(options)

	out := make(chan WatchEvent)

	containers := make(map[string]bool)

	parsePodStatus := func(pod *v1.Pod) {
		if pod.Status.Phase == v1.PodSucceeded {
			log.Println("Status: succeded")
			out <- WatchEvent{Completed, pod, "", ""}
		} else if pod.Status.Phase == v1.PodFailed {
			log.Println("Status: failed")
			out <- WatchEvent{Failed, pod, "", ""}
		} else {
			for _, container := range pod.Status.ContainerStatuses {
				if container.State.Running != nil {
					_, present := containers[container.Name]
					if !present {
						out <- WatchEvent{Added, pod, container.Name, ""}
						containers[container.Name] = true
					}
				} else if container.State.Terminated != nil {
					_, present := containers[container.Name]
					if present {
						out <- WatchEvent{Removed, pod, container.Name, ""}
						containers[container.Name] = false
					}
					if container.State.Terminated.ExitCode != 0 {
						out <- WatchEvent{Failed, pod, container.Name, container.State.Terminated.Message}
					}
				}
			}
		}
	}

	go func() {
		for {
			select {
			case e := <-watcher.ResultChan():
				if e.Object == nil {
					// Closed because of error
					return
				}

				pod := e.Object.(*v1.Pod)

				switch e.Type {
				case watch.Added, watch.Modified:
					parsePodStatus(pod)
				case watch.Deleted:
					out <- WatchEvent{Deleted, pod, "", ""}
				case watch.Error:
					log.Printf("Pod error")
				}
			case <-ctx.Done():
				watcher.Stop()
				close(out)
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-time.After(watchPollInterval):
				pod, err := c.CoreV1().Pods(watchPod.Namespace).Get(watchPod.Name, metav1.GetOptions{})
				if pod != nil {
					parsePodStatus(pod)
				} else {
					if err != nil {
						log.Println("Error polling pod status: %s", err.Error())
					} else {
						log.Println("No pod status")
					}
				}
			case <-ctx.Done():
				return
			}
		}

	}()

	return out, nil
}

func TailLogs(ctx context.Context, c kubernetes.Interface, pod *v1.Pod, container string) {
	pods := c.Core().Pods(pod.Namespace)

	req := pods.GetLogs(pod.Name, &v1.PodLogOptions{
		Container: container,
		Follow:    true,
	})

	closed := make(chan struct{})

	go func() {

		stream, err := req.Stream()

		if err != nil {
			log.Fatalf("Error opening log stream for pod %s", pod.Name)
		}

		defer stream.Close()

		go func() {
			<-closed
			stream.Close()
		}()

		go func() {
			<-ctx.Done()
			close(closed)
		}()

		reader := bufio.NewReader(stream)

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}

			str := string(line)

			log.Printf("[%s/%s]: %s", pod.Name, container, str)
		}
	}()
}
