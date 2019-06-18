package pipeline

import (
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func parseTimeOrDie(ts string) metav1.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		panic(err)
	}
	return metav1.Time{Time: t}
}

var testRunFlags = &runCmdFlagsStruct{
	TailLogs:           false,
	DeletePollInterval: 1 * time.Millisecond,
	StartTimeout:       1 * time.Hour,
}

func createPodStatus(phase v1.PodPhase, containers map[string]bool) v1.PodStatus {
	containerStatuses := make([]v1.ContainerStatus, len(containers))
	for container, running := range containers {
		var state v1.ContainerState
		if running {
			state = v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: parseTimeOrDie("2015-04-22T11:49:32Z"),
				},
			}
		} else {
			state = v1.ContainerState{
				Terminated: &v1.ContainerStateTerminated{
					ExitCode: 1,
				},
			}
		}
		containerStatuses = append(containerStatuses, v1.ContainerStatus{
			Name:         container,
			State:        state,
			Ready:        true,
			RestartCount: 0,
			Image:        "test.com/test",
			ImageID:      "docker://b6b9a86dc06aa1361357ca1b105feba961f6a4145adca6c54e142c0be0fe87b0",
			ContainerID:  "docker://b6b9a86dc06aa1361357ca1b105feba961f6a4145adca6c54e142c0be0fe87b0",
		})
	}

	return v1.PodStatus{
		Phase: phase,
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodReady,
				Status: v1.ConditionTrue,
			},
		},
		ContainerStatuses: containerStatuses,
	}
}

func TestRunPipelineSuccess(t *testing.T) {
	client := fake.NewSimpleClientset()
	clientset = client

	fakeWatch := watch.NewFake()
	client.PrependWatchReactor("pods", ktesting.DefaultWatchReactor(fakeWatch, nil))

	deleted := make(map[string]int)

	client.PrependReactor("delete", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		a := action.(ktesting.DeleteAction)
		name := a.GetName()
		deleted[name] += 1
		if deleted[name] < 2 {
			return true, nil, nil
		} else {
			fakeWatch.Reset()
			return true, nil, k8errors.NewNotFound(v1.Resource("pods"), name)
		}
	})

	created := make(map[string]int)

	client.PrependReactor("create", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		a := action.(ktesting.CreateAction)
		object := a.GetObject()
		pod := object.(*v1.Pod)
		created[pod.Name] += 1
		go func() {
			p := pod.DeepCopy()
			p.Status = createPodStatus(v1.PodRunning, map[string]bool{pod.Name + "/main": true, pod.Name + "/paddle": true})
			fakeWatch.Add(p)
			time.Sleep(100 * time.Millisecond)
			p = p.DeepCopy()
			p.Status = createPodStatus(v1.PodSucceeded, map[string]bool{pod.Name + "/main": true, pod.Name + "/paddle": true})
			fakeWatch.Modify(p)
		}()
		return true, object, nil
	})

	runPipeline("test/sample_steps_passing.yml", testRunFlags)

	expectPods := [2]string{"sample-steps-passing-version1-step1-master", "sample-steps-passing-version1a-step2-master"}

	for _, p := range expectPods {
		if deleted[p] != 3 {
			t.Errorf("expected delete of "+p+" to be called twice, got %i", deleted[p])
		}
		if created[p] != 1 {
			t.Errorf("expected create of "+p+" to be called once, got %i", created[p])
		}
	}
}

func TestRunPipelineFailure(t *testing.T) {
	origLogFatalf := logFatalf

	// after this test, replace the original fatal function
	defer func() { logFatalf = origLogFatalf }()

	errors := []string{}
	logFatalf = func(format string, args ...interface{}) {
		if len(args) > 0 {
			errors = append(errors, fmt.Sprintf(format, args))
		} else {
			errors = append(errors, format)
		}
	}

	client := fake.NewSimpleClientset()
	clientset = client

	fakeWatch := watch.NewFake()
	client.PrependWatchReactor("pods", ktesting.DefaultWatchReactor(fakeWatch, nil))

	client.PrependReactor("delete", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		fakeWatch.Reset()
		return true, nil, k8errors.NewNotFound(v1.Resource("pods"), action.(ktesting.DeleteAction).GetName())
	})

	client.PrependReactor("create", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		a := action.(ktesting.CreateAction)
		object := a.GetObject()
		pod := object.(*v1.Pod)
		go func() {
			p := pod.DeepCopy()
			p.Status = createPodStatus(v1.PodRunning, map[string]bool{pod.Name + "/main": true, pod.Name + "/paddle": false})
			fakeWatch.Add(p)
		}()
		return true, object, nil
	})

	runPipeline("test/sample_steps_passing.yml", testRunFlags)

	if len(errors) != 2 {
		t.Errorf("expected two errors, actual %v", len(errors))
	}
}

func TestRunPipelineStartTimeout(t *testing.T) {
	origLogFatalf := logFatalf

	// after this test, replace the original fatal function
	defer func() { logFatalf = origLogFatalf }()

	errors := []string{}
	logFatalf = func(format string, args ...interface{}) {
		if len(args) > 0 {
			errors = append(errors, fmt.Sprintf(format, args))
		} else {
			errors = append(errors, format)
		}
	}

	client := fake.NewSimpleClientset()
	clientset = client

	fakeWatch := watch.NewFake()
	client.PrependWatchReactor("pods", ktesting.DefaultWatchReactor(fakeWatch, nil))

	client.PrependReactor("delete", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		fakeWatch.Reset()
		return true, nil, k8errors.NewNotFound(v1.Resource("pods"), action.(ktesting.DeleteAction).GetName())
	})

	client.PrependReactor("create", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		a := action.(ktesting.CreateAction)
		object := a.GetObject()
		time.Sleep(200 * time.Millisecond)
		return true, object, nil
	})

	flags := *testRunFlags
	flags.StartTimeout = 1 * time.Millisecond

	runPipeline("test/sample_steps_passing.yml", &flags)

	if len(errors) != 2 {
		t.Errorf("expected two errors, actual %v", len(errors))
	}
	msg := "[paddle] [Timed out waiting for pod to start. Cluster might not have sufficient resources.]"
	for _, err := range errors {
		if err != msg {
			t.Errorf("Expected timeout error, got %s", err)
		}
	}
}
