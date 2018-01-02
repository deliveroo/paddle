package pipeline

import (
	"context"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"testing"
	"time"
)

func TestWatch(t *testing.T) {
	client := fake.NewSimpleClientset()
	clientset = client

	fakeWatch := watch.NewFake()
	client.PrependWatchReactor("pods", ktesting.DefaultWatchReactor(fakeWatch, nil))

	ctx, _ := context.WithCancel(context.Background())
	watchPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testpod",
			Namespace: "testnamespace",
		},
	}
	ch, _ := Watch(ctx, client, watchPod)

	events := make([]WatchEvent, 0)

	go func() {
		for {
			e := <-ch
			events = append(events, e)
		}
	}()

	p := watchPod.DeepCopy()
	p.Status = createPodStatus(v1.PodRunning, map[string]bool{"foo": true, "bar": true})
	fakeWatch.Add(p)
	p = p.DeepCopy()
	p.Status = createPodStatus(v1.PodSucceeded, map[string]bool{"foo": true, "bar": true})
	fakeWatch.Modify(p)
	p = p.DeepCopy()
	p.Status = createPodStatus(v1.PodRunning, map[string]bool{"foo": true, "bar": false})
	fakeWatch.Modify(p)
	fakeWatch.Stop()

	time.Sleep(100 * time.Millisecond)

	types := []WatchEventType{Added, Added, Completed, Removed, Failed}

	if len(events) != len(types) {
		t.Errorf("Expected %i events, got %i", len(types), len(events))
	}

	for i, et := range types {
		if events[i].Type != et {
			t.Errorf("Event %i type is %v, expected %v", i, events[i].Type, et)
		}
	}
}
