// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package dao

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/derailed/k9s/internal"
	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/render"
	"github.com/derailed/k9s/internal/slogs"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	mv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

var _ Accessor = (*Namespace)(nil)

// Namespace represents a namespace resource.
type Namespace struct {
	Resource
}

// List returns a collection of namespaces with their pods stats.
func (n *Namespace) List(ctx context.Context, ns string) ([]runtime.Object, error) {
	oo, err := n.Resource.List(ctx, ns)
	if err != nil {
		return oo, err
	}

	var (
		podsIn map[string][]*v1.Pod
		alloc  v1.ResourceList
		usage  map[string]v1.ResourceList
	)
	if shouldCountPods, _ := ctx.Value(internal.KeyPodCounting).(bool); shouldCountPods {
		pods, err := n.getFactory().List(client.PodGVR, client.BlankNamespace, false, labels.Everything())
		if err != nil {
			slog.Error("Unable to list pods", slogs.Error, err)
		} else {
			podsIn = podsByNamespace(pods)
		}
		if nn, err := n.getFactory().List(client.NodeGVR, client.BlankNamespace, false, labels.Everything()); err == nil {
			alloc = clusterAllocatable(nn)
		}
	}
	if withMx, ok := ctx.Value(internal.KeyWithMetrics).(bool); ok && withMx {
		if mx, err := client.DialMetrics(n.Client()).FetchPodsMetrics(ctx, client.BlankNamespace); err == nil {
			usage = usageByNamespace(mx)
		}
	}

	res := make([]runtime.Object, 0, len(oo))
	for _, o := range oo {
		u, ok := o.(*unstructured.Unstructured)
		if !ok {
			return res, fmt.Errorf("expecting *unstructured.Unstructured but got `%T", o)
		}
		nws := render.NamespaceWithStats{Raw: u, Allocatable: alloc}
		if podsIn != nil {
			pp := podsIn[u.GetName()]
			nws.Pods = podStats(pp)
			nws.Requests, nws.Limits = AllocatedResources(pp)
		}
		if usage != nil {
			nws.MX = usage[u.GetName()]
			if nws.MX == nil {
				nws.MX = v1.ResourceList{}
			}
		}
		res = append(res, &nws)
	}

	return res, nil
}

// podsByNamespace groups pods by namespace.
func podsByNamespace(oo []runtime.Object) map[string][]*v1.Pod {
	pp := make(map[string][]*v1.Pod)
	for _, o := range oo {
		u, ok := o.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		var po v1.Pod
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &po); err != nil {
			slog.Error("Unable to convert pod", slogs.Error, err)
			continue
		}
		pp[po.Namespace] = append(pp[po.Namespace], &po)
	}

	return pp
}

// podStats computes the given pods health breakdown.
func podStats(pp []*v1.Pod) *render.PodStats {
	var (
		p render.Pod
		s render.PodStats
	)
	for _, po := range pp {
		s.Add(p.Health(po))
	}

	return &s
}

// usageByNamespace sums up pods resources usage per namespace.
func usageByNamespace(mx *mv1beta1.PodMetricsList) map[string]v1.ResourceList {
	uu := make(map[string]v1.ResourceList)
	for i := range mx.Items {
		pmx := &mx.Items[i]
		total, ok := uu[pmx.Namespace]
		if !ok {
			total = v1.ResourceList{}
			uu[pmx.Namespace] = total
		}
		for j := range pmx.Containers {
			addResources(total, pmx.Containers[j].Usage)
		}
	}

	return uu
}

// clusterAllocatable sums up all nodes allocatable resources.
func clusterAllocatable(oo []runtime.Object) v1.ResourceList {
	total := v1.ResourceList{}
	for _, o := range oo {
		u, ok := o.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		var no v1.Node
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &no); err != nil {
			slog.Error("Unable to convert node", slogs.Error, err)
			continue
		}
		addResources(total, no.Status.Allocatable)
	}

	return total
}
