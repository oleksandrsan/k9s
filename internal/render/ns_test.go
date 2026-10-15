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
