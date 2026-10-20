package brackets

import (
	"reflect"
	"testing"
)

// Feeding matches through a reused Match value gives the same result as
// the stage-based functions.
func TestTallyMatchesStageFunctions(t *testing.T) {
	f := Swiss{}
	st := planOne(t, f, seeded(9), Options{Rounds: 4})
	for round := 0; round < 3; round++ {
		for i := range st.Matches {
			if st.Matches[i].Status == Pending {
				if _, err := ReportAt(f, st, i, Result{Winner: st.Matches[i].Slots[1].ID, Scores: []int{1, 3}}); err != nil {
					t.Fatal(err)
				}
				break
			}
		}
	}
	feed := func() *Tally {
		tl := NewTally(st.Participants, st.Byes)
		var m Match
		for i := range st.Matches {
			m = st.Matches[i]
			m.Slots = append([]Slot(nil), st.Matches[i].Slots...)
			tl.Add(&m)
			m.Slots[0].ID = "garbage" // the tally must not keep references
		}
		return tl
	}
	if got, want := feed().Standings(), Standings(st); !reflect.DeepEqual(got, want) {
		t.Fatalf("standings differ:\n%v\n%v", got, want)
	}
	got, err := f.AdvanceTally(st, feed())
	if err != nil {
		t.Fatal(err)
	}
	want, _ := f.Advance(st, nil)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("advance differs:\n%+v\n%+v", got, want)
	}
	tl := feed()
	if tl.MaxRound() != st.CurrentRound || tl.AllResolved() || tl.RoundResolved(0) {
		t.Fatalf("round tracking: max %d", tl.MaxRound())
	}
}

func TestParentAndMoves(t *testing.T) {
	for _, c := range [][5]int{{1, 1, 2, 1, 0}, {1, 2, 2, 1, 1}, {2, 3, 3, 2, 0}, {3, 4, 4, 2, 1}} {
		r, n, s := SingleElimination{}.Parent(c[0], c[1])
		if r != c[2] || n != c[3] || s != c[4] {
			t.Fatalf("Parent(%d,%d) = %d,%d,%d", c[0], c[1], r, n, s)
		}
	}
	st := planOne(t, DoubleElimination{}, seeded(8), Options{})
	m := *st.Match("upper-r1-m1")
	m.Winner = "p8"
	mv := DoubleElimination{}.Moves(&m)
	if mv.Loser != "p1" || mv.LoserSlot != 0 || mv.WinnerTo.Match != "upper-r2-m1" || mv.LoserTo.Match != "lower-r2-m1" || mv.Final {
		t.Fatalf("moves %+v", mv)
	}
	gf := *st.Match("grand-final")
	gf.Winner = "p2"
	if mv := (DoubleElimination{}).Moves(&gf); !mv.Final || mv.Winner != "p2" {
		t.Fatalf("grand final moves %+v", mv)
	}
	m.Winner = ""
	if mv := (DoubleElimination{}).Moves(&m); mv.WinnerTo != nil || mv.LoserTo != nil {
		t.Fatalf("no winner should move nobody: %+v", mv)
	}
}
