// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package dao

import (
	"testing"

	"github.com/derailed/k9s/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPodStatsByNamespace(t *testing.T) {
	oo := []runtime.Object{
		makeNSPod(t, "ns1", v1.PodRunning, true, nil),
		makeNSPod(t, "ns1", v1.PodPending, false, nil),
		makeNSPod(t, "ns1", v1.PodRunning, false, &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}),
		makeNSPod(t, "ns1", v1.PodSucceeded, false, nil),
		makeNSPod(t, "ns2", v1.PodRunning, true, nil),
	}

	ss := podStatsByNamespace(oo)
	assert.Len(t, ss, 2)
	assert.Equal(t, render.PodStats{Total: 3, Pending: 1, Faults: 1}, *ss["ns1"])
	assert.Equal(t, render.PodStats{Total: 1}, *ss["ns2"])
}

// Helpers...

func makeNSPod(t *testing.T, ns string, phase v1.PodPhase, ready bool, waiting *v1.ContainerStateWaiting) *unstructured.Unstructured {
	t.Helper()

	cond := v1.ConditionFalse
	if ready {
		cond = v1.ConditionTrue
	}
	cs := v1.ContainerStatus{Name: "c1", Ready: ready}
	switch {
	case waiting != nil:
		cs.State.Waiting = waiting
	case phase == v1.PodSucceeded:
		cs.State.Terminated = &v1.ContainerStateTerminated{Reason: "Completed"}
	case phase == v1.PodRunning:
		cs.State.Running = &v1.ContainerStateRunning{}
	}
	po := v1.Pod{
		Spec: v1.PodSpec{Containers: []v1.Container{{Name: "c1"}}},
		Status: v1.PodStatus{
			Phase:      phase,
			Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: cond}},
		},
	}
	if phase != v1.PodPending {
		po.Status.ContainerStatuses = []v1.ContainerStatus{cs}
	}
	po.Namespace = ns
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&po)
	require.NoError(t, err)

	return &unstructured.Unstructured{Object: m}
}
