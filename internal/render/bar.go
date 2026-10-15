// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package render

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/derailed/k9s/internal/config"
	"github.com/derailed/k9s/internal/model1"
	"github.com/derailed/tcell/v2"
)

const (
	barWidth     = 5
	barEmptyRune = "░"
	// barStdColor renders a gauge using the row color.
	barStdColor = "-"
)

// barEighths holds partial block glyphs, indexed by eighths of a cell.
var barEighths = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// percRX captures a leading percentage and any trailing decorations (ie deltas).
var percRX = regexp.MustCompile(`^\s*(\d+)%?(.*)$`)

// BarStyle tracks gauges severity thresholds and colors.
type BarStyle struct {
	Thresholds           config.Threshold
	WarnColor, CritColor tcell.Color
}

var barStyle atomic.Pointer[BarStyle]

func init() {
	SetBarStyle(BarStyle{
		Thresholds: config.NewThreshold(),
		WarnColor:  tcell.ColorOrangeRed,
		CritColor:  tcell.ColorRed,
	})
}

// SetBarStyle sets gauges thresholds and colors ie on skin or config changes.
func SetBarStyle(s BarStyle) {
	barStyle.Store(&s)
}

// CPUBarDecorator renders a cpu percentage column as a gauge.
var CPUBarDecorator = percBarDecorator(config.CPU)

// MEMBarDecorator renders a memory percentage column as a gauge.
var MEMBarDecorator = percBarDecorator(config.MEM)

// percBarDecorator renders a percentage column as a gauge followed by its value,
// colored per the given resource thresholds.
// Values above 100 (ie overcommitted limits) render a full gauge.
func percBarDecorator(res string) model1.DecoratorFunc {
	return func(field string) string {
		mm := percRX.FindStringSubmatch(field)
		if len(mm) != 3 {
			return field
		}
		p, err := strconv.Atoi(mm[1])
		if err != nil {
			return field
		}

		return fmt.Sprintf("[%s::]%s[-::] %3d%%%s", barStyle.Load().color(res, p), percBar(p, barWidth), p, mm[2])
	}
}

func (s *BarStyle) color(res string, p int) string {
	//nolint:exhaustive
	switch s.Thresholds.LevelFor(res, min(p, 100)) {
	case config.SeverityHigh:
		return tagColor(s.CritColor)
	case config.SeverityMedium:
		return tagColor(s.WarnColor)
	default:
		return barStdColor
	}
}

// tagColor converts a color to a style tag color.
func tagColor(c tcell.Color) string {
	if c == tcell.ColorDefault || !c.Valid() {
		return barStdColor
	}

	return fmt.Sprintf("#%06x", c.Hex())
}

// percBar renders a gauge of width cells with an 1/8 cell resolution.
func percBar(p, width int) string {
	p = max(0, min(p, 100))
	eighths := p * width * 8 / 100
	full, part := eighths/8, eighths%8

	var b strings.Builder
	b.WriteString(strings.Repeat("█", full))
	if part > 0 {
		b.WriteString(barEighths[part])
		full++
	}
	b.WriteString(strings.Repeat(barEmptyRune, width-full))

	return b.String()
}
