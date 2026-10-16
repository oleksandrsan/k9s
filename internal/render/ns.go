// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package render

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/derailed/k9s/internal/client"
	"github.com/derailed/k9s/internal/model1"
	"github.com/derailed/k9s/internal/slogs"
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"golang.org/x/exp/slog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var defaultNSHeader = model1.Header{
	model1.HeaderColumn{Name: colName},
	model1.HeaderColumn{Name: colStatus},
	model1.HeaderColumn{Name: "PODS", Attrs: model1.Attrs{Align: tview.AlignRight}},
	model1.HeaderColumn{Name: "PENDING", Attrs: model1.Attrs{Align: tview.AlignRight}},
	model1.HeaderColumn{Name: "FAULTS", Attrs: model1.Attrs{Align: tview.AlignRight}},
	model1.HeaderColumn{Name: colCPU, Attrs: model1.Attrs{Align: tview.AlignRight, MX: true}},
	model1.HeaderColumn{Name: "CPU/R", Attrs: model1.Attrs{Align: tview.AlignRight, Wide: true}},
	model1.HeaderColumn{Name: "CPU/L", Attrs: model1.Attrs{Align: tview.AlignRight, Wide: true}},
	model1.HeaderColumn{Name: "%CPU/R", Attrs: model1.Attrs{Align: tview.AlignRight, MX: true, Decorator: PercBarDecorator}},
	model1.HeaderColumn{Name: "%CPU/C", Attrs: model1.Attrs{Align: tview.AlignRight, Decorator: PercBarDecorator}},
	model1.HeaderColumn{Name: colMEM, Attrs: model1.Attrs{Align: tview.AlignRight, MX: true}},
	model1.HeaderColumn{Name: "MEM/R", Attrs: model1.Attrs{Align: tview.AlignRight, Wide: true}},
	model1.HeaderColumn{Name: "MEM/L", Attrs: model1.Attrs{Align: tview.AlignRight, Wide: true}},
	model1.HeaderColumn{Name: "%MEM/R", Attrs: model1.Attrs{Align: tview.AlignRight, MX: true, Decorator: PercBarDecorator}},
	model1.HeaderColumn{Name: "%MEM/C", Attrs: model1.Attrs{Align: tview.AlignRight, Decorator: PercBarDecorator}},
	model1.HeaderColumn{Name: colLabels, Attrs: model1.Attrs{Wide: true}},
	model1.HeaderColumn{Name: colValid, Attrs: model1.Attrs{Wide: true}},
	model1.HeaderColumn{Name: colAge, Attrs: model1.Attrs{Time: true}},
}

// Namespace renders a K8s Namespace to screen.
type Namespace struct {
	Base
}

// ColorerFunc colors a resource row.
func (Namespace) ColorerFunc() model1.ColorerFunc {
	return func(ns string, h model1.Header, re *model1.RowEvent) tcell.Color {
		c := model1.DefaultColorer(ns, h, re)
		if c == model1.ErrColor {
			return c
		}
		if re.Kind == model1.EventUpdate {
			c = model1.StdColor
		}
		if strings.Contains(strings.TrimSpace(re.Row.Fields[0]), "*") {
			c = model1.HighlightColor
		}

		return c
	}
}

// Header returns a header row.
func (n Namespace) Header(_ string) model1.Header {
	return n.doHeader(defaultNSHeader)
}

// Render renders a K8s resource to screen.
func (n Namespace) Render(o any, _ string, row *model1.Row) error {
	nws, err := asNamespaceWithStats(o)
	if err != nil {
		return err
	}
	if e := n.defaultRow(nws, row); e != nil {
		return e
	}
	if n.specs.isEmpty() {
		return nil
	}

	cols, err := n.specs.realize(nws.Raw, defaultNSHeader, row)
	cols.hydrateRow(row)

	return err
}

func (n Namespace) defaultRow(nws *NamespaceWithStats, r *model1.Row) error {
	var ns v1.Namespace
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(nws.Raw.Object, &ns)
	if err != nil {
		return err
	}

	pods, pending, faults := NAValue, NAValue, NAValue
	if ps := nws.Pods; ps != nil {
		pods, pending, faults = strconv.Itoa(ps.Total), strconv.Itoa(ps.Pending), strconv.Itoa(ps.Faults)
	}

	cpu, mem := nws.utilization()

	r.ID = client.MetaFQN(&ns.ObjectMeta)
	r.Fields = model1.Fields{
		ns.Name,
		string(ns.Status.Phase),
		pods,
		pending,
		faults,
		cpu.usage,
		cpu.req,
		cpu.lim,
		cpu.pReq,
		cpu.pCluster,
		mem.usage,
		mem.req,
		mem.lim,
		mem.pReq,
		mem.pCluster,
		mapToStr(ns.Labels),
		AsStatus(n.diagnose(ns.Status.Phase, nws.Pods)),
		ToAge(ns.GetCreationTimestamp()),
	}

	return nil
}

// Healthy checks component health.
func (n Namespace) Healthy(_ context.Context, o any) error {
	nws, err := asNamespaceWithStats(o)
	if err != nil {
		slog.Error("Unexpected namespace type", slogs.Type, fmt.Sprintf("%T", o))
		return nil
	}
	var ns v1.Namespace
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(nws.Raw.Object, &ns)
	if err != nil {
		slog.Error("Failed to convert Unstructured to Namespace", slogs.Type, fmt.Sprintf("%T", o), slog.String("error", err.Error()))
		return nil
	}

	return n.diagnose(ns.Status.Phase, nws.Pods)
}

func (Namespace) diagnose(phase v1.NamespacePhase, ps *PodStats) error {
	if phase != v1.NamespaceActive && phase != v1.NamespaceTerminating {
		return errors.New("namespace not ready")
	}
	if ps != nil && ps.Faults > 0 {
		return fmt.Errorf("%d faulty pod(s)", ps.Faults)
	}

	return nil
}

// ----------------------------------------------------------------------------
// Helpers...

// PodStats tracks a namespace pods health breakdown.
type PodStats struct {
	// Total tracks all pods but the ones ran to completion.
	Total, Pending, Faults int
}

// Add accounts for a pod in the given health category.
func (s *PodStats) Add(h PodHealth) {
	if h == PodDone {
		return
	}
	s.Total++
	//nolint:exhaustive
	switch h {
	case PodPending:
		s.Pending++
	case PodFaulty:
		s.Faults++
	}
}

// NamespaceWithStats represents a namespace with its pods stats.
type NamespaceWithStats struct {
	Raw *unstructured.Unstructured
	// Pods tracks the namespace pods health. Nil when unknown.
	Pods *PodStats
	// MX tracks the namespace pods resources usage. Nil when unknown.
	MX v1.ResourceList
	// Requests and Limits track resources allocated by the namespace pods. Nil when unknown.
	Requests, Limits v1.ResourceList
	// Allocatable tracks the cluster allocatable resources. Nil when unknown.
	Allocatable v1.ResourceList
}

type nsUtilization struct {
	usage, req, lim, pReq, pCluster string
}

// utilization renders the namespace cpu and memory utilization.
// Usage is measured against requests and requests against the cluster allocatable resources.
func (n *NamespaceWithStats) utilization() (cpu, mem nsUtilization) {
	cpu = nsUtilization{NAValue, NAValue, NAValue, NAValue, NAValue}
	mem = cpu
	if n.MX != nil {
		cpu.usage, mem.usage = toMc(n.MX.Cpu().MilliValue()), toMi(n.MX.Memory().Value())
	}
	if n.Requests == nil {
		return
	}
	rcpu, rmem := n.Requests.Cpu().MilliValue(), n.Requests.Memory().Value()
	cpu.req, mem.req = toMc(rcpu), toMi(rmem)
	cpu.lim, mem.lim = toMc(n.Limits.Cpu().MilliValue()), toMi(n.Limits.Memory().Value())
	if n.MX != nil {
		cpu.pReq = client.ToPercentageStr(n.MX.Cpu().MilliValue(), rcpu)
		mem.pReq = client.ToPercentageStr(n.MX.Memory().Value(), rmem)
	}
	if n.Allocatable != nil {
		cpu.pCluster = client.ToPercentageStr(rcpu, n.Allocatable.Cpu().MilliValue())
		mem.pCluster = client.ToPercentageStr(rmem, n.Allocatable.Memory().Value())
	}

	return
}

// GetObjectKind returns a schema object.
func (*NamespaceWithStats) GetObjectKind() schema.ObjectKind {
	return nil
}

// DeepCopyObject returns a container copy.
func (n *NamespaceWithStats) DeepCopyObject() runtime.Object {
	return n
}

func asNamespaceWithStats(o any) (*NamespaceWithStats, error) {
	switch v := o.(type) {
	case *NamespaceWithStats:
		return v, nil
	case *unstructured.Unstructured:
		return &NamespaceWithStats{Raw: v}, nil
	default:
		return nil, fmt.Errorf("expected NamespaceWithStats, but got %T", o)
	}
}
