package brackets_test

import (
	"fmt"

	"github.com/Aquilabot/brackets"
)

// A six-player single-elimination bracket: the two top seeds get byes, the
// higher seed wins every match.
func Example() {
	players := []brackets.Participant{
		{ID: "ana", Seed: 1}, {ID: "ben", Seed: 2}, {ID: "cleo", Seed: 3},
		{ID: "dan", Seed: 4}, {ID: "eve", Seed: 5}, {ID: "fay", Seed: 6},
	}
	f := brackets.SingleElimination{}
	stages, err := f.Plan(players, brackets.Options{})
	if err != nil {
		panic(err)
	}
	st := &stages[0]
	for _, m := range st.Matches {
		switch {
		case m.Bye:
			fmt.Printf("%s: bye for %s\n", m.ID, m.Winner)
		case m.Round == 1:
			fmt.Printf("%s: %s vs %s\n", m.ID, m.Home(), m.Away())
		}
	}

	for !st.Finished {
		for i := range st.Matches {
			m := &st.Matches[i]
			if m.Status != brackets.Pending || m.Home() == "" || m.Away() == "" {
				continue
			}
			if _, err := brackets.ReportAt(f, st, i, brackets.Result{Winner: m.Home(), Scores: []int{2, 0}}); err != nil {
				panic(err)
			}
		}
	}
	fmt.Println("champion:", st.Winner)
	// Output:
	// r1-m1: bye for ana
	// r1-m2: dan vs eve
	// r1-m3: bye for ben
	// r1-m4: cleo vs fay
	// champion: ana
}

// Swiss rounds are generated one at a time, as soon as the previous round
// is complete.
func ExampleSwiss() {
	f := brackets.Swiss{}
	stages, err := f.Plan([]brackets.Participant{
		{ID: "ana", Seed: 1}, {ID: "ben", Seed: 2}, {ID: "cleo", Seed: 3},
		{ID: "dan", Seed: 4}, {ID: "eve", Seed: 5},
	}, brackets.Options{Rounds: 3})
	if err != nil {
		panic(err)
	}
	st := &stages[0]
	for i := 0; i < len(st.Matches); i++ { // new rounds are appended
		m := &st.Matches[i]
		if _, err := brackets.ReportAt(f, st, i, brackets.Result{Winner: m.Away()}); err != nil {
			panic(err)
		}
	}
	for _, b := range st.Byes {
		fmt.Printf("round %d bye: %s\n", b.Round, b.Participant)
	}
	for _, s := range st.Standings[:3] {
		fmt.Printf("%d. %s %d-%d\n", s.Rank, s.ID, s.Wins, s.Losses)
	}
	// Output:
	// round 1 bye: eve
	// round 2 bye: cleo
	// round 3 bye: ben
	// 1. ana 2-1
	// 2. ben 2-1
	// 3. dan 2-1
}

func ExampleRegistry() {
	reg := brackets.DefaultRegistry()
	f, err := reg.Get("double_elimination")
	if err != nil {
		panic(err)
	}
	stages, _ := f.Plan([]brackets.Participant{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}, brackets.Options{})
	for _, m := range stages[0].Matches {
		fmt.Println(m.ID, m.Bracket, "best of", m.BestOf)
	}
	fmt.Println(reg.Keys())
	// Output:
	// upper-r1-m1 upper best of 3
	// lower-r1-m1 lower best of 1
	// lower-r2-m1 lower best of 3
	// grand-final grand_final best of 5
	// [double_elimination ffa_league group_stage multi_stage round_robin single_elimination swiss]
}
