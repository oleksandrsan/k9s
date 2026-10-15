// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestAllocatedResources(t *testing.T) {
	always := v1.ContainerRestartPolicyAlways

	uu := map[string]struct {
		pp           []*v1.Pod
		reqs, limits v1.ResourceList
	}{
		"empty": {
			reqs:   v1.ResourceList{},
			limits: v1.ResourceList{},
		},

		"no-resources": {
			pp:     []*v1.Pod{makeAllocPod(v1.PodRunning, makeCo(nil, nil))},
			reqs:   v1.ResourceList{},
			limits: v1.ResourceList{},
		},

		"multi": {
			pp: []*v1.Pod{
				makeAllocPod(v1.PodRunning,
					makeCo(makeRL("100m", "100Mi"), makeRL("200m", "200Mi")),
					makeCo(makeRL("50m", "50Mi"), nil),
				),
				makeAllocPod(v1.PodPending, makeCo(makeRL("1", "1Gi"), makeRL("2", "2Gi"))),
			},
			reqs:   makeRL("1150m", "1174Mi"),
			limits: makeRL("2200m", "2248Mi"),
		},

		"terminated": {
			pp: []*v1.Pod{
				makeAllocPod(v1.PodRunning, makeCo(makeRL("100m", "100Mi"), nil)),
				makeAllocPod(v1.PodSucceeded, makeCo(makeRL("1", "1Gi"), nil)),
				makeAllocPod(v1.PodFailed, makeCo(makeRL("1", "1Gi"), nil)),
			},
			reqs:   makeRL("100m", "100Mi"),
			limits: v1.ResourceList{},
		},

		"init-containers": {
			pp: []*v1.Pod{
				func() *v1.Pod {
					po := makeAllocPod(v1.PodRunning, makeCo(makeRL("100m", "100Mi"), nil))
					po.Spec.InitContainers = []v1.Container{makeCo(makeRL("1", "10Mi"), nil)}
					return po
				}(),
			},
			reqs:   makeRL("1", "100Mi"),
			limits: v1.ResourceList{},
		},

		"sidecars": {
			pp: []*v1.Pod{
				func() *v1.Pod {
					po := makeAllocPod(v1.PodRunning, makeCo(makeRL("100m", "100Mi"), nil))
					sc := makeCo(makeRL("50m", "50Mi"), nil)
					sc.RestartPolicy = &always
					po.Spec.InitContainers = []v1.Container{sc}
					return po
				}(),
			},
			reqs:   makeRL("150m", "150Mi"),
			limits: v1.ResourceList{},
		},
	}

	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			reqs, limits := AllocatedResources(u.pp)
			assertRL(t, u.reqs, reqs)
			assertRL(t, u.limits, limits)
		})
	}
}

func TestPodsByNode(t *testing.T) {
	oo := []runtime.Object{
		makeUnstructuredPod(t, "p1", "n1"),
		makeUnstructuredPod(t, "p2", "n2"),
		makeUnstructuredPod(t, "p3", "n1"),
		makeUnstructuredPod(t, "p4", ""),
	}

	pp := podsByNode(oo)
	assert.Len(t, pp, 2)
	assert.Len(t, pp["n1"], 2)
	assert.Len(t, pp["n2"], 1)
}

// Helpers...

func assertRL(t *testing.T, e, a v1.ResourceList) {
	t.Helper()
	assert.Len(t, a, len(e))
	for k, q := range e {
		v, ok := a[k]
		assert.True(t, ok, k)
		assert.Zero(t, q.Cmp(v), "%s: expected %s but got %s", k, q.String(), v.String())
	}
}

func makeRL(cpu, mem string) v1.ResourceList {
	return v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse(cpu),
		v1.ResourceMemory: resource.MustParse(mem),
	}
}

func makeCo(reqs, limits v1.ResourceList) v1.Container {
	return v1.Container{
		Name: "c",
		Resources: v1.ResourceRequirements{
			Requests: reqs,
			Limits:   limits,
		},
	}
}

func makeAllocPod(phase v1.PodPhase, cc ...v1.Container) *v1.Pod {
	return &v1.Pod{
		Spec:   v1.PodSpec{Containers: cc},
		Status: v1.PodStatus{Phase: phase},
	}
}

func makeUnstructuredPod(t *testing.T, name, node string) *unstructured.Unstructured {
	t.Helper()
	po := v1.Pod{Spec: v1.PodSpec{NodeName: node}}
	po.Name = name
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&po)
	assert.NoError(t, err)

	return &unstructured.Unstructured{Object: m}
}
