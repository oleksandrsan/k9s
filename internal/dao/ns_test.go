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
	mv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestPodStatsByNamespace(t *testing.T) {
	oo := []runtime.Object{
		makeNSPod(t, "ns1", v1.PodRunning, true, nil),
		makeNSPod(t, "ns1", v1.PodPending, false, nil),
		makeNSPod(t, "ns1", v1.PodRunning, false, &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}),
		makeNSPod(t, "ns1", v1.PodSucceeded, false, nil),
		makeNSPod(t, "ns2", v1.PodRunning, true, nil),
	}

	pp := podsByNamespace(oo)
	assert.Len(t, pp, 2)
	assert.Equal(t, render.PodStats{Total: 3, Pending: 1, Faults: 1}, *podStats(pp["ns1"]))
	assert.Equal(t, render.PodStats{Total: 1}, *podStats(pp["ns2"]))
	assert.Equal(t, render.PodStats{}, *podStats(pp["ns3"]))
}

func TestUsageByNamespace(t *testing.T) {
	mx := mv1beta1.PodMetricsList{
		Items: []mv1beta1.PodMetrics{
			makePodMX("ns1", makeRL("100m", "10Mi"), makeRL("50m", "5Mi")),
			makePodMX("ns1", makeRL("200m", "20Mi")),
			makePodMX("ns2", makeRL("1", "1Gi")),
		},
	}

	uu := usageByNamespace(&mx)
	assert.Len(t, uu, 2)
	assertRL(t, makeRL("350m", "35Mi"), uu["ns1"])
	assertRL(t, makeRL("1", "1Gi"), uu["ns2"])
}

func TestClusterAllocatable(t *testing.T) {
	oo := []runtime.Object{
		makeAllocNode(t, makeRL("4", "8Gi")),
		makeAllocNode(t, makeRL("2", "4Gi")),
	}

	assertRL(t, makeRL("6", "12Gi"), clusterAllocatable(oo))
}

// Helpers...

func makePodMX(ns string, rr ...v1.ResourceList) mv1beta1.PodMetrics {
	pmx := mv1beta1.PodMetrics{}
	pmx.Namespace = ns
	for _, r := range rr {
		pmx.Containers = append(pmx.Containers, mv1beta1.ContainerMetrics{Usage: r})
	}

	return pmx
}

func makeAllocNode(t *testing.T, alloc v1.ResourceList) *unstructured.Unstructured {
	t.Helper()
	no := v1.Node{Status: v1.NodeStatus{Allocatable: alloc}}
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&no)
	require.NoError(t, err)

	return &unstructured.Unstructured{Object: m}
}

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
