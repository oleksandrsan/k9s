// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"context"
	"strconv"

	"github.com/derailed/k9s/internal"
	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/model1"
	"github.com/derailed/k9s/internal/render"
	"github.com/derailed/k9s/internal/ui"
	"github.com/derailed/tcell/v2"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	favNSIndicator     = "+"
	defaultNSIndicator = "(*)"
)

// Namespace represents a namespace viewer.
type Namespace struct {
	ResourceViewer
}

// NewNamespace returns a new viewer.
func NewNamespace(gvr *client.GVR) ResourceViewer {
	n := Namespace{
		ResourceViewer: NewBrowser(gvr),
	}
	n.GetTable().SetDecorateFn(n.decorate)
	n.GetTable().SetEnterFn(n.switchNs)
	n.AddBindKeysFn(n.bindKeys)
	n.SetContextFn(n.nsContext)

	return &n
}

func (n *Namespace) nsContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, internal.KeyPodCounting, !n.App().Config.K9s.DisablePodCounting)
}

func (n *Namespace) bindKeys(aa *ui.KeyActions) {
	aa.Bulk(ui.KeyMap{
		ui.KeyU: ui.NewKeyAction("Use", n.useNsCmd, true),
	})
}

func (n *Namespace) switchNs(app *App, _ ui.Tabular, _ *client.GVR, path string) {
	n.useNamespace(path)
	_, ns := client.Namespaced(path)
	app.gotoResource(client.PodGVR.String()+" "+ns, "", false, true)
}

func (n *Namespace) useNsCmd(*tcell.EventKey) *tcell.EventKey {
	path := n.GetTable().GetSelectedItem()
	if path == "" {
		return nil
	}
	n.useNamespace(path)

	return nil
}

func (n *Namespace) useNamespace(fqn string) {
	_, ns := client.Namespaced(fqn)
	if client.CleanseNamespace(n.App().Config.ActiveNamespace()) == ns {
		return
	}
	if err := n.App().switchNS(ns); err != nil {
		n.App().Flash().Err(err)
		return
	}
	if err := n.App().Config.SetActiveNamespace(ns); err != nil {
		n.App().Flash().Err(err)
		return
	}
}

func (n *Namespace) decorate(td *model1.TableData) {
	if n.App().Conn() == nil || td.RowCount() == 0 {
		return
	}
	// checks if all ns is in the list if not add it.
	if _, ok := td.FindRow(client.NamespaceAll); !ok {
		td.AddRow(model1.RowEvent{
			Kind: model1.EventUnchanged,
			Row: model1.Row{
				ID:     client.NamespaceAll,
				Fields: allNSFields(td),
			},
		},
		)
	}

	var (
		favs     = sets.New(n.App().Config.FavNamespaces()...)
		activeNS = n.App().Config.ActiveNamespace()
	)
	td.RowsRange(func(i int, re model1.RowEvent) bool {
		_, n := client.Namespaced(re.Row.ID)
		if favs.Has(n) {
			re.Row.Fields[0] += favNSIndicator
		}
		if n == activeNS {
			re.Row.Fields[0] += defaultNSIndicator
		}
		re.Kind = model1.EventUnchanged
		td.SetRow(i, re)
		return true
	})
}

// allNSFields builds the all namespaces row, totaling pods stats and utilization across namespaces.
func allNSFields(td *model1.TableData) model1.Fields {
	h := td.Header()
	ff := make(model1.Fields, len(h))
	for i, c := range h {
		switch c.Name {
		case "NAME":
			ff[i] = client.NamespaceAll
		case "STATUS":
			ff[i] = "Active"
		case "PODS", "PENDING", "FAULTS", "CPU", "CPU/R", "CPU/L", "MEM", "MEM/R", "MEM/L":
			ff[i] = sumColumn(td, i)
		}
	}
	for pct, cols := range map[string][2]string{"%CPU/R": {"CPU", "CPU/R"}, "%MEM/R": {"MEM", "MEM/R"}} {
		i, ok1 := h.IndexOf(pct, true)
		v, ok2 := h.IndexOf(cols[0], true)
		dv, ok3 := h.IndexOf(cols[1], true)
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		ff[i] = percentOf(ff[v], ff[dv])
	}

	return ff
}

func percentOf(v, dv string) string {
	n, err1 := strconv.Atoi(v)
	d, err2 := strconv.Atoi(dv)
	if err1 != nil || err2 != nil {
		return render.NAValue
	}

	return client.ToPercentageStr(int64(n), int64(d))
}

func sumColumn(td *model1.TableData, col int) string {
	var total int
	ok := true
	td.RowsRange(func(_ int, re model1.RowEvent) bool {
		if col >= len(re.Row.Fields) {
			return true
		}
		v, err := strconv.Atoi(re.Row.Fields[col])
		if err != nil {
			ok = false
			return false
		}
		total += v
		return true
	})
	if !ok {
		return render.NAValue
	}

	return strconv.Itoa(total)
}
