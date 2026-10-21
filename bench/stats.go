package main

import (
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
)

// bootstrapResamples is enough for a stable 95% interval at the scale of
// this benchmark (tens of tasks, tens of trials) and cheap enough to run on
// every table.
const bootstrapResamples = 2000

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	if n := len(s); n%2 == 1 {
		return s[n/2]
	} else {
		return (s[n/2-1] + s[n/2]) / 2
	}
}

// quantile returns the p-quantile (0 to 1) of sorted values by linear
// interpolation, the same definition as R's type 7 and NumPy's default.
func quantile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}

type interval struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

// bootstrapCI is the 95% percentile bootstrap interval of stat over xs,
// from a generator seeded by seed, so the same run-id always yields the same
// tables. One or zero samples give a degenerate interval at the point value.
func bootstrapCI(xs []float64, stat func([]float64) float64, seed uint64) interval {
	if len(xs) < 2 {
		v := stat(xs)
		return interval{v, v}
	}
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	sample := make([]float64, len(xs))
	stats := make([]float64, bootstrapResamples)
	for i := range stats {
		for j := range sample {
			sample[j] = xs[rng.IntN(len(xs))]
		}
		stats[i] = stat(sample)
	}
	slices.Sort(stats)
	return interval{quantile(stats, 0.025), quantile(stats, 0.975)}
}

// seedFrom turns a run-id (or any label) into a bootstrap seed, so the
// report is a pure function of the data and the run's name.
func seedFrom(label string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(label))
	return h.Sum64()
}

// passRate is the fraction of trials that passed, and passAll is pass^k:
// whether every trial passed. Both are 0 and false for an empty cell.
func passRate(trials []Trial) float64 {
	if len(trials) == 0 {
		return 0
	}
	n := 0
	for _, t := range trials {
		if t.Passed {
			n++
		}
	}
	return float64(n) / float64(len(trials))
}

func passAll(trials []Trial) bool {
	if len(trials) == 0 {
		return false
	}
	for _, t := range trials {
		if !t.Passed {
			return false
		}
	}
	return true
}
