// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package render_test

import (
	"testing"

	cfg "github.com/derailed/k9s/internal/config"
	"github.com/derailed/k9s/internal/render"
	"github.com/derailed/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestBarDecorator(t *testing.T) {
	render.SetBarStyle(render.BarStyle{
		Thresholds: cfg.Threshold{
			cfg.CPU: {Warn: 70, Critical: 90},
			cfg.MEM: {Warn: 50, Critical: 80},
		},
		WarnColor: tcell.NewHexColor(0xffa500),
		CritColor: tcell.NewHexColor(0xff0000),
	})
	t.Cleanup(resetBarStyle)

	uu := map[string]struct {
		dec  func(string) string
		f, e string
	}{
		"na": {
			dec: render.CPUBarDecorator,
			f:   "n/a",
			e:   "n/a",
		},
		"blank": {
			dec: render.CPUBarDecorator,
			f:   "",
			e:   "",
		},
		"zero": {
			dec: render.CPUBarDecorator,
			f:   "0",
			e:   "[-::]░░░░░   0%[-::]",
		},
		"partial": {
			dec: render.CPUBarDecorator,
			f:   "25",
			e:   "[-::]█▎░░░  25%[-::]",
		},
		"cpu-warn": {
			dec: render.CPUBarDecorator,
			f:   "70",
			e:   "[#ffa500::]███▌░  70%[-::]",
		},
		"cpu-crit": {
			dec: render.CPUBarDecorator,
			f:   "95",
			e:   "[#ff0000::]████▊  95%[-::]",
		},
		"mem-warn": {
			dec: render.MEMBarDecorator,
			f:   "60",
			e:   "[#ffa500::]███░░  60%[-::]",
		},
		"mem-crit": {
			dec: render.MEMBarDecorator,
			f:   "80",
			e:   "[#ff0000::]████░  80%[-::]",
		},
		"full": {
			dec: render.CPUBarDecorator,
			f:   "100",
			e:   "[#ff0000::]█████ 100%[-::]",
		},
		"overcommit": {
			dec: render.CPUBarDecorator,
			f:   "250",
			e:   "[#ff0000::]█████ 250%[-::]",
		},
		"plain-crit": {
			dec: render.PercBarDecorator,
			f:   "95",
			e:   "[-::]████▊  95%[-::]",
		},
		"plain-over": {
			dec: render.PercBarDecorator,
			f:   "250",
			e:   "[-::]█████ 250%[-::]",
		},
		"delta": {
			dec: render.CPUBarDecorator,
			f:   "42[red::b]↑",
			e:   "[-::]██░░░  42%[-::][red::b]↑",
		},
	}

	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, u.e, u.dec(u.f))
		})
	}
}

func TestBarDecoratorDefaultColors(t *testing.T) {
	render.SetBarStyle(render.BarStyle{
		Thresholds: cfg.NewThreshold(),
		WarnColor:  tcell.ColorDefault,
		CritColor:  tcell.ColorDefault,
	})
	t.Cleanup(resetBarStyle)

	assert.Equal(t, "[-::]████▉  99%[-::]", render.CPUBarDecorator("99"))
}

func resetBarStyle() {
	render.SetBarStyle(render.BarStyle{
		Thresholds: cfg.NewThreshold(),
		WarnColor:  tcell.ColorOrangeRed,
		CritColor:  tcell.ColorRed,
	})
}
