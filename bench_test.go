package brackets

import (
	"strconv"
	"testing"
)

// Benchmark scenarios used for the README "Performance" table. Keep the
// names stable so results stay comparable across versions.

var benchSizes = []int{8, 64, 512, 4096}

func benchName(n int) string { return "n=" + strconv.Itoa(n) }

func BenchmarkPlan(b *testing.B) {
	cases := []struct {
		name  string
		f     Format
		opts  func(n int) Options
		sizes []int
	}{
		{"single_elimination", SingleElimination{}, func(int) Options { return Options{} }, benchSizes},
		{"double_elimination", DoubleElimination{}, func(int) Options { return Options{} }, benchSizes},
		{"swiss", Swiss{}, func(int) Options { return Options{Pairing: Greedy} }, benchSizes},
		{"round_robin_group", GroupStage{}, func(n int) Options {
			return Options{GroupSize: n, MatchSize: 2, Seeding: Sequential, Partial: PartialAllow}
		}, []int{8, 64, 512}},
		{"ffa_league", FFALeague{}, func(int) Options { return Options{} }, []int{64, 512, 4096}},
	}
	for _, c := range cases {
		for _, n := range c.sizes {
			ps := seeded(n)
			opts := c.opts(n)
			b.Run(c.name+"/"+benchName(n), func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					if _, err := c.f.Plan(ps, opts); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// runStage reports every match in slice order (home wins) until the stage
// finishes, like a caller persisting one result at a time.
func runStage(b *testing.B, f Format, st *Stage) {
	for i := 0; i < len(st.Matches) && !st.Finished; i++ {
		m := &st.Matches[i]
		if m.Status != Pending {
			continue
		}
		if _, err := ReportAt(f, st, i, Result{Winner: m.Slots[0].ID, Scores: []int{2, 1}}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRun(b *testing.B) {
	cases := []struct {
		name  string
		f     Format
		opts  Options
		sizes []int
	}{
		{"single_elimination", SingleElimination{}, Options{}, benchSizes},
		{"double_elimination", DoubleElimination{}, Options{}, benchSizes},
		{"swiss", Swiss{}, Options{Pairing: Greedy}, []int{8, 64, 512}},
		{"swiss_12_rounds", Swiss{}, Options{Rounds: 12, Pairing: Greedy}, []int{64, 512}},
		{"swiss_12_rounds_avoid_rematches", Swiss{}, Options{Rounds: 12}, []int{64, 512}},
	}
	for _, c := range cases {
		for _, n := range c.sizes {
			ps := seeded(n)
			b.Run(c.name+"/"+benchName(n), func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					stages, err := c.f.Plan(ps, c.opts)
					if err != nil {
						b.Fatal(err)
					}
					runStage(b, c.f, &stages[0])
				}
			})
		}
	}
}

// completedRound returns a Swiss stage whose round `round` just finished.
func completedRound(b *testing.B, n, rounds, round int, pairing Pairing) *Stage {
	f := Swiss{}
	stages, err := f.Plan(seeded(n), Options{Rounds: rounds, Pairing: pairing})
	if err != nil {
		b.Fatal(err)
	}
	st := &stages[0]
	for i := 0; i < len(st.Matches); i++ {
		m := &st.Matches[i]
		if m.Status != Pending {
			continue
		}
		if m.Round == round && i == len(st.Matches)-1 {
			m.Status, m.Winner = Finished, m.Slots[0].ID
			return st
		}
		if _, err := ReportAt(f, st, i, Result{Winner: m.Slots[0].ID}); err != nil {
			b.Fatal(err)
		}
	}
	b.Fatal("round not reached")
	return nil
}

// BenchmarkSwissNextRound measures one Advance that closes a round and
// pairs the next one: standings, history and pairing.
func BenchmarkSwissNextRound(b *testing.B) {
	for _, n := range benchSizes {
		for _, p := range []Pairing{Greedy, AvoidRematches} {
			name := "greedy"
			if p == AvoidRematches {
				name = "avoid_rematches"
			}
			st := completedRound(b, n, 12, 6, p)
			last := &st.Matches[len(st.Matches)-1]
			b.Run(name+"/"+benchName(n), func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					adv, err := Swiss{}.Advance(st, last)
					if err != nil || len(adv.NewMatches) == 0 {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkStandings(b *testing.B) {
	for _, n := range benchSizes {
		st := completedRound(b, n, 12, 6, Greedy)
		b.Run(benchName(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				Standings(st)
			}
		})
	}
}
