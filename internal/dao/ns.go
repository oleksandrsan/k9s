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

	var stats map[string]*render.PodStats
	if shouldCountPods, _ := ctx.Value(internal.KeyPodCounting).(bool); shouldCountPods {
		pods, err := n.getFactory().List(client.PodGVR, client.BlankNamespace, false, labels.Everything())
		if err != nil {
			slog.Error("Unable to list pods", slogs.Error, err)
		} else {
			stats = podStatsByNamespace(pods)
		}
	}

	res := make([]runtime.Object, 0, len(oo))
	for _, o := range oo {
		u, ok := o.(*unstructured.Unstructured)
		if !ok {
			return res, fmt.Errorf("expecting *unstructured.Unstructured but got `%T", o)
		}
		nws := render.NamespaceWithStats{Raw: u}
		if stats != nil {
			nws.Pods = stats[u.GetName()]
			if nws.Pods == nil {
				nws.Pods = new(render.PodStats)
			}
		}
		res = append(res, &nws)
	}

	return res, nil
}

// podStatsByNamespace computes pods health breakdown per namespace.
func podStatsByNamespace(oo []runtime.Object) map[string]*render.PodStats {
	var (
		p     render.Pod
		stats = make(map[string]*render.PodStats)
	)
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
		s, ok := stats[po.Namespace]
		if !ok {
			s = new(render.PodStats)
			stats[po.Namespace] = s
		}
		s.Add(p.Health(&po))
	}

	return stats
}
