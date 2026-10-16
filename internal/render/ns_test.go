// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package render_test

import (
	"testing"

	"github.com/derailed/k9s/internal/model1"
	"github.com/derailed/k9s/internal/render"
	"github.com/derailed/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNSColorer(t *testing.T) {
	uu := map[string]struct {
		re model1.RowEvent
		e  tcell.Color
	}{
		"add": {
			re: model1.RowEvent{
				Kind: model1.EventAdd,
				Row: model1.Row{
					Fields: model1.Fields{
						"blee",
						"Active",
					},
				},
			},
			e: model1.AddColor,
		},
		"update": {
			re: model1.RowEvent{
				Kind: model1.EventUpdate,
				Row: model1.Row{
					Fields: model1.Fields{
						"blee",
						"Active",
					},
				},
			},
			e: model1.StdColor,
		},
		"decorator": {
			re: model1.RowEvent{
				Kind: model1.EventAdd,
				Row: model1.Row{
					Fields: model1.Fields{
						"blee*",
						"Active",
					},
				},
			},
			e: model1.HighlightColor,
		},
	}

	h := model1.Header{
		model1.HeaderColumn{Name: "NAME"},
		model1.HeaderColumn{Name: "STATUS"},
	}

	var r render.Namespace
	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, u.e, r.ColorerFunc()("", h, &u.re))
		})
	}
}

func TestNamespaceRender(t *testing.T) {
	c := render.Namespace{}
	r := model1.NewRow(3)

	require.NoError(t, c.Render(load(t, "ns"), "-", &r))
	assert.Equal(t, "-/kube-system", r.ID)
	assert.Equal(t, model1.Fields{"kube-system", "Active"}, r.Fields[:2])
}

func TestNamespaceRenderStats(t *testing.T) {
	uu := map[string]struct {
		ps    *render.PodStats
		e     model1.Fields
		valid string
	}{
		"unknown": {
			e: model1.Fields{"kube-system", "Active", "n/a", "n/a", "n/a"},
		},
		"empty": {
			ps: &render.PodStats{},
			e:  model1.Fields{"kube-system", "Active", "0", "0", "0"},
		},
		"healthy": {
			ps: &render.PodStats{Total: 5, Pending: 2},
			e:  model1.Fields{"kube-system", "Active", "5", "2", "0"},
		},
		"faulty": {
			ps:    &render.PodStats{Total: 5, Pending: 1, Faults: 3},
			e:     model1.Fields{"kube-system", "Active", "5", "1", "3"},
			valid: "3 faulty pod(s)",
		},
	}

	var n render.Namespace
	h := n.Header("")
	idx, ok := h.IndexOf("VALID", true)
	require.True(t, ok)
	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			r := model1.NewRow(8)
			require.NoError(t, n.Render(&render.NamespaceWithStats{Raw: load(t, "ns"), Pods: u.ps}, "-", &r))
			assert.Equal(t, u.e, r.Fields[:5])
			assert.Equal(t, u.valid, r.Fields[idx])
		})
	}
}

func TestNamespaceRenderUtilization(t *testing.T) {
	var (
		usage = v1.ResourceList{v1.ResourceCPU: resource.MustParse("250m"), v1.ResourceMemory: resource.MustParse("512Mi")}
		reqs  = v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}
		lims  = v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("2Gi")}
		alloc = v1.ResourceList{v1.ResourceCPU: resource.MustParse("8"), v1.ResourceMemory: resource.MustParse("16Gi")}
	)

	uu := map[string]struct {
		nws render.NamespaceWithStats
		e   map[string]string
	}{
		"full": {
			nws: render.NamespaceWithStats{MX: usage, Requests: reqs, Limits: lims, Allocatable: alloc},
			e: map[string]string{
				"CPU": "250", "CPU/R": "1000", "CPU/L": "2000", "%CPU/R": "25", "%CPU/C": "12",
				"MEM": "512", "MEM/R": "1024", "MEM/L": "2048", "%MEM/R": "50", "%MEM/C": "6",
			},
		},
		"no-metrics": {
			nws: render.NamespaceWithStats{Requests: reqs, Limits: lims, Allocatable: alloc},
			e: map[string]string{
				"CPU": "n/a", "CPU/R": "1000", "%CPU/R": "n/a", "%CPU/C": "12",
				"MEM": "n/a", "MEM/R": "1024", "%MEM/R": "n/a", "%MEM/C": "6",
			},
		},
		"no-requests": {
			nws: render.NamespaceWithStats{MX: usage, Requests: v1.ResourceList{}, Limits: v1.ResourceList{}, Allocatable: alloc},
			e: map[string]string{
				"CPU": "250", "CPU/R": "0", "%CPU/R": "n/a", "%CPU/C": "0",
			},
		},
		"no-pods": {
			nws: render.NamespaceWithStats{MX: usage},
			e: map[string]string{
				"CPU": "250", "CPU/R": "n/a", "CPU/L": "n/a", "%CPU/R": "n/a", "%CPU/C": "n/a",
			},
		},
		"no-nodes": {
			nws: render.NamespaceWithStats{MX: usage, Requests: reqs, Limits: lims},
			e: map[string]string{
				"%CPU/R": "25", "%CPU/C": "n/a", "%MEM/C": "n/a",
			},
		},
	}

	var n render.Namespace
	h := n.Header("")
	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			u.nws.Raw = load(t, "ns")
			r := model1.NewRow(18)
			require.NoError(t, n.Render(&u.nws, "-", &r))
			for col, e := range u.e {
				idx, ok := h.IndexOf(col, true)
				require.True(t, ok, col)
				assert.Equal(t, e, r.Fields[idx], col)
			}
		})
	}
}
