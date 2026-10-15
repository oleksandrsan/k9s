// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package render

import (
	"regexp"
	"time"

	v1 "k8s.io/api/core/v1"
)

// PodHealth tracks a pod health category.
type PodHealth int

const (
	// PodHealthy tracks a running and ready pod.
	PodHealthy PodHealth = iota

	// PodPending tracks a pod in transit ie starting up or shutting down.
	PodPending

	// PodFaulty tracks a pod requiring attention.
	PodFaulty

	// PodDone tracks a pod that ran to completion.
	PodDone
)

// terminatingGrace tracks how long a pod may linger past its deletion time
// before it is considered stuck.
const terminatingGrace = time.Minute

// initProgressRX matches init containers still making progress ie Init:0/2.
var initProgressRX = regexp.MustCompile(`^Init:\d+/\d+$`)

// Health categorizes a pod using the same checks as the pods view.
func (p *Pod) Health(po *v1.Pod) PodHealth {
	return p.health(po, time.Now())
}

func (p *Pod) health(po *v1.Pod, now time.Time) PodHealth {
	dt := po.DeletionTimestamp
	phase := p.Phase(dt, &po.Spec, &po.Status)
	if phase == Completed || (dt == nil && po.Status.Phase == v1.PodSucceeded) {
		return PodDone
	}

	cr, _, _, _ := p.ContainerStats(po.Status.ContainerStatuses)
	ct := len(po.Status.ContainerStatuses)
	icr, ict, _ := p.initContainerStats(po.Spec.InitContainers, po.Status.InitContainerStatuses)
	rgr, rgt := p.readinessGateStats(&po.Spec, &po.Status)
	if p.diagnose(phase, cr+icr, ct+ict, hasPodReadyCondition(po.Status.Conditions), rgr, rgt) == nil {
		return PodHealthy
	}

	switch {
	case phase == Terminating:
		if dt != nil && now.Before(dt.Add(terminatingGrace)) {
			return PodPending
		}
	case phase == PhasePending, phase == PhaseContainerCreating, phase == PhasePodInitializing:
		return PodPending
	case initProgressRX.MatchString(phase):
		return PodPending
	}

	return PodFaulty
}
