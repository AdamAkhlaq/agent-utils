package main

import (
	"math"
	"strings"
	"testing"
)

func TestMeanMedian(t *testing.T) {
	tests := []struct {
		name       string
		xs         []float64
		wantMean   float64
		wantMedian float64
	}{
		{"empty", nil, 0, 0},
		{"one", []float64{3}, 3, 3},
		{"odd", []float64{5, 1, 3}, 3, 3},
		{"even", []float64{4, 1, 3, 2}, 2.5, 2.5},
		{"skewed", []float64{1, 1, 1, 100}, 25.75, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mean(tt.xs); got != tt.wantMean {
				t.Errorf("mean = %v, want %v", got, tt.wantMean)
			}
			if got := median(tt.xs); got != tt.wantMedian {
				t.Errorf("median = %v, want %v", got, tt.wantMedian)
			}
		})
	}
}

func TestQuantile(t *testing.T) {
	sorted := []float64{10, 20, 30, 40}
	tests := []struct {
		p    float64
		want float64
	}{
		{0, 10}, {1, 40}, {0.5, 25}, {0.25, 17.5}, {0.975, 39.25},
	}
	for _, tt := range tests {
		if got := quantile(sorted, tt.p); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("quantile(%v) = %v, want %v", tt.p, got, tt.want)
		}
	}
	if got := quantile(nil, 0.5); got != 0 {
		t.Errorf("quantile(empty) = %v, want 0", got)
	}
}

func TestBootstrapCI(t *testing.T) {
	xs := []float64{0.8, 1.1, 0.9, 1.3, 0.7, 1.0, 1.2, 0.95, 1.05, 0.85}
	seed := seedFrom("run-1")

	ci := bootstrapCI(xs, median, seed)
	if ci.Low > median(xs) || ci.High < median(xs) {
		t.Errorf("interval %+v does not contain the median %v", ci, median(xs))
	}
	if ci.Low >= ci.High {
		t.Errorf("interval %+v is not widening", ci)
	}
	if again := bootstrapCI(xs, median, seed); again != ci {
		t.Errorf("same seed gave %+v then %+v", ci, again)
	}
	// The mean varies continuously with the resample, so a different seed
	// moves the interval; the median of ten values often does not.
	if a, b := bootstrapCI(xs, mean, seed), bootstrapCI(xs, mean, seedFrom("run-2")); a == b {
		t.Errorf("a different seed gave the identical mean interval %+v", a)
	}

	constant := []float64{2, 2, 2, 2}
	if ci := bootstrapCI(constant, mean, seed); ci != (interval{2, 2}) {
		t.Errorf("constant sample gave %+v, want [2, 2]", ci)
	}
	if ci := bootstrapCI([]float64{7}, mean, seed); ci != (interval{7, 7}) {
		t.Errorf("single sample gave %+v, want [7, 7]", ci)
	}
	if ci := bootstrapCI(nil, mean, seed); ci != (interval{0, 0}) {
		t.Errorf("empty sample gave %+v, want [0, 0]", ci)
	}
}

func TestPassRate(t *testing.T) {
	trials := func(passed ...bool) []Trial {
		var ts []Trial
		for _, p := range passed {
			ts = append(ts, Trial{Passed: p})
		}
		return ts
	}
	tests := []struct {
		name     string
		trials   []Trial
		wantRate float64
		wantAll  bool
	}{
		{"empty", nil, 0, false},
		{"all pass", trials(true, true, true), 1, true},
		{"one fails", trials(true, false, true, true), 0.75, false},
		{"all fail", trials(false, false), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := passRate(tt.trials); got != tt.wantRate {
				t.Errorf("passRate = %v, want %v", got, tt.wantRate)
			}
			if got := passAll(tt.trials); got != tt.wantAll {
				t.Errorf("passAll = %v, want %v", got, tt.wantAll)
			}
		})
	}
}

func TestSeedFrom(t *testing.T) {
	if seedFrom("a") == seedFrom("b") {
		t.Error("different labels gave the same seed")
	}
	label := "run"
	if first, second := seedFrom(label), seedFrom(strings.Clone(label)); first != second {
		t.Error("the same label gave different seeds")
	}
}
