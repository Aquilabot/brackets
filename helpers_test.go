package brackets

import (
	"strconv"
	"testing"
)

// seeded returns participants "p1".."pn" with seeds 1..n.
func seeded(n int) []Participant {
	ps := make([]Participant, n)
	for i := range ps {
		ps[i] = Participant{ID: "p" + strconv.Itoa(i+1), Seed: i + 1}
	}
	return ps
}

// unseeded returns participants "p1".."pn" without seeds.
func unseeded(n int) []Participant {
	ps := make([]Participant, n)
	for i := range ps {
		ps[i] = Participant{ID: "p" + strconv.Itoa(i+1)}
	}
	return ps
}

func planOne(t testing.TB, f Format, ps []Participant, opts Options) *Stage {
	t.Helper()
	stages, err := f.Plan(ps, opts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(stages) != 1 {
		t.Fatalf("want 1 stage, got %d", len(stages))
	}
	return &stages[0]
}

// playable returns the index of the first pending match with every slot
// filled, or -1.
func playable(st *Stage) int {
	for i := range st.Matches {
		m := &st.Matches[i]
		if m.Status != Pending {
			continue
		}
		ok := len(m.Slots) >= 2
		for _, s := range m.Slots {
			if s.ID == "" {
				ok = false
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

// playAll reports every playable match with pick choosing the winner, until
// the stage finishes or nothing is playable. It returns the number of
// reports.
func playAll(t testing.TB, f Format, st *Stage, pick func(m *Match) Result) int {
	t.Helper()
	n := 0
	for !st.Finished {
		i := playable(st)
		if i < 0 {
			break
		}
		if _, err := Report(f, st, st.Matches[i].ID, pick(&st.Matches[i])); err != nil {
			t.Fatalf("Report %s: %v", st.Matches[i].ID, err)
		}
		n++
	}
	return n
}

func homeWins(m *Match) Result { return Result{Winner: m.Slots[0].ID, Scores: []int{2, 1}} }

// higherSeedWins picks the participant with the lowest "pN" number.
func higherSeedWins(m *Match) Result {
	best, bestN := "", 1<<30
	for _, s := range m.Slots {
		n, _ := strconv.Atoi(s.ID[1:])
		if n < bestN {
			best, bestN = s.ID, n
		}
	}
	return Result{Winner: best}
}
