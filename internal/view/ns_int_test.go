// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"testing"

	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/model1"
	"github.com/derailed/k9s/internal/render"
	"github.com/stretchr/testify/assert"
)

func TestAllNSFields(t *testing.T) {
	var n render.Namespace
	h := n.Header("")
	uu := map[string]struct {
		rows []model1.Fields
		e    model1.Fields
	}{
		"totals": {
			rows: []model1.Fields{
				{"ns1", "Active", "5", "1", "2", "100", "400", "800", "25", "10", "256", "512", "1024", "50", "5", "", "2 faulty pod(s)", "1d"},
				{"ns2", "Active", "3", "0", "1", "300", "400", "0", "75", "10", "0", "512", "512", "0", "5", "", "1 faulty pod(s)", "2d"},
			},
			e: model1.Fields{client.NamespaceAll, "Active", "8", "1", "3", "400", "800", "800", "50", "", "256", "1024", "1536", "25", "", "", "", ""},
		},
		"unknown": {
			rows: []model1.Fields{
				{"ns1", "Active", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "", "", "1d"},
			},
			e: model1.Fields{client.NamespaceAll, "Active", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "", "n/a", "n/a", "n/a", "n/a", "", "", "", ""},
		},
	}

	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			rr := model1.NewRowEvents(len(u.rows))
			for _, ff := range u.rows {
				rr.Add(model1.RowEvent{Row: model1.Row{ID: ff[0], Fields: ff}})
			}
			td := model1.NewTableDataWithRows(client.NsGVR, h, rr)

			assert.Equal(t, u.e, allNSFields(td))
		})
	}
}
