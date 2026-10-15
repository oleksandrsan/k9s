// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodHealth(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	uu := map[string]struct {
		po *v1.Pod
		e  PodHealth
	}{
		"running-ready": {
			po: makeHealthPod(v1.PodRunning, true, runningCS(true)),
			e:  PodHealthy,
		},
		"running-not-ready": {
			po: makeHealthPod(v1.PodRunning, false, runningCS(false)),
			e:  PodFaulty,
		},
		"crashloop": {
			po: makeHealthPod(v1.PodRunning, false, waitingCS("CrashLoopBackOff")),
			e:  PodFaulty,
		},
		"image-pull": {
			po: makeHealthPod(v1.PodPending, false, waitingCS("ImagePullBackOff")),
			e:  PodFaulty,
		},
		"pending": {
			po: makeHealthPod(v1.PodPending, false),
			e:  PodPending,
		},
		"creating": {
			po: makeHealthPod(v1.PodPending, false, waitingCS("ContainerCreating")),
			e:  PodPending,
		},
		"init-progress": {
			po: func() *v1.Pod {
				po := makeHealthPod(v1.PodPending, false, waitingCS("PodInitializing"))
				po.Spec.InitContainers = []v1.Container{{Name: "i1"}}
				po.Status.InitContainerStatuses = []v1.ContainerStatus{{Name: "i1", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}}}
				return po
			}(),
			e: PodPending,
		},
		"init-crashloop": {
			po: func() *v1.Pod {
				po := makeHealthPod(v1.PodPending, false, waitingCS("PodInitializing"))
				po.Spec.InitContainers = []v1.Container{{Name: "i1"}}
				po.Status.InitContainerStatuses = []v1.ContainerStatus{waitingCS("CrashLoopBackOff")}
				return po
			}(),
			e: PodFaulty,
		},
		"succeeded": {
			po: makeHealthPod(v1.PodSucceeded, false, terminatedCS("Completed", 0)),
			e:  PodDone,
		},
		"failed": {
			po: makeHealthPod(v1.PodFailed, false, terminatedCS("Error", 1)),
			e:  PodFaulty,
		},
		"evicted": {
			po: func() *v1.Pod {
				po := makeHealthPod(v1.PodFailed, false)
				po.Status.Reason = "Evicted"
				return po
			}(),
			e: PodFaulty,
		},
		"terminating": {
			po: func() *v1.Pod {
				po := makeHealthPod(v1.PodRunning, true, runningCS(true))
				po.DeletionTimestamp = &metav1.Time{Time: now.Add(10 * time.Second)}
				return po
			}(),
			e: PodPending,
		},
		"terminating-stuck": {
			po: func() *v1.Pod {
				po := makeHealthPod(v1.PodRunning, true, runningCS(true))
				po.DeletionTimestamp = &metav1.Time{Time: now.Add(-5 * time.Minute)}
				return po
			}(),
			e: PodFaulty,
		},
	}

	var p Pod
	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, u.e, p.health(u.po, now))
		})
	}
}

func TestPodStatsAdd(t *testing.T) {
	var s PodStats
	for _, h := range []PodHealth{PodHealthy, PodPending, PodFaulty, PodFaulty, PodDone} {
		s.Add(h)
	}

	assert.Equal(t, PodStats{Total: 4, Pending: 1, Faults: 2}, s)
}

// Helpers...

func makeHealthPod(phase v1.PodPhase, ready bool, cc ...v1.ContainerStatus) *v1.Pod {
	cond := v1.ConditionFalse
	if ready {
		cond = v1.ConditionTrue
	}
	spec := v1.PodSpec{}
	for i := range cc {
		spec.Containers = append(spec.Containers, v1.Container{Name: cc[i].Name})
	}

	return &v1.Pod{
		Spec: spec,
		Status: v1.PodStatus{
			Phase:             phase,
			Conditions:        []v1.PodCondition{{Type: v1.PodReady, Status: cond}},
			ContainerStatuses: cc,
		},
	}
}

func runningCS(ready bool) v1.ContainerStatus {
	return v1.ContainerStatus{
		Name:  "c1",
		Ready: ready,
		State: v1.ContainerState{Running: &v1.ContainerStateRunning{}},
	}
}

func waitingCS(reason string) v1.ContainerStatus {
	return v1.ContainerStatus{
		Name:  "c1",
		State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: reason}},
	}
}

func terminatedCS(reason string, code int32) v1.ContainerStatus {
	return v1.ContainerStatus{
		Name:  "c1",
		State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{Reason: reason, ExitCode: code}},
	}
}
