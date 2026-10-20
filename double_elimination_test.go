package brackets

import (
	"errors"
	"testing"
)

func TestDoubleEliminationSixteen(t *testing.T) {
	st := planOne(t, DoubleElimination{}, seeded(16), Options{})
	if len(st.Matches) != 22 {
		t.Fatalf("want 22 matches, got %d", len(st.Matches))
	}
	count := func(b Bracket, round int) int {
		n := 0
		for _, m := range st.Matches {
			if m.Bracket == b && m.BracketRound == round {
				n++
			}
		}
		return n
	}
	if count(UpperBracket, 1) != 4 || count(LowerBracket, 1) != 4 {
		t.Fatalf("round 1: upper %d lower %d", count(UpperBracket, 1), count(LowerBracket, 1))
	}
	lr1 := st.Match("lower-r1-m1")
	if lr1 == nil || lr1.BestOf != 1 || !lr1.Elimination {
		t.Fatalf("lower-r1-m1 = %+v", lr1)
	}
	gf := st.Match("grand-final")
	if gf == nil || gf.BestOf != 5 || gf.Bracket != GrandFinal || gf.Round != 7 {
		t.Fatalf("grand final = %+v", gf)
	}
	// Top half of the seeds starts upper, bottom half lower.
	if m := st.Match("upper-r1-m1"); m.Home() != "p1" || m.Away() != "p8" {
		t.Fatalf("upper-r1-m1 = %s vs %s", m.Home(), m.Away())
	}
	if m := st.Match("lower-r1-m1"); m.Home() != "p9" || m.Away() != "p16" {
		t.Fatalf("lower-r1-m1 = %s vs %s", m.Home(), m.Away())
	}
	// Upper losers drop into the even lower rounds.
	if l := st.Match("upper-r1-m2").LoserTo; l == nil || *l != (Link{Match: "lower-r2-m2", Slot: 1, Index: 12}) {
		t.Fatalf("upper-r1-m2 loser link = %+v", l)
	}
	if l := st.Match("upper-r3-m1").WinnerTo; l == nil || (l.Match != "grand-final" || l.Slot != 0 || st.Matches[l.Index].ID != "grand-final") {
		t.Fatalf("upper final winner link = %+v", l)
	}
}

func TestDoubleEliminationPropagatesWinnerAndLoser(t *testing.T) {
	f := DoubleElimination{}
	st := planOne(t, f, seeded(8), Options{})
	adv, err := Report(f, st, "upper-r1-m1", Result{Winner: "p1", Scores: []int{2, 0}})
	if err != nil {
		t.Fatal(err)
	}
	if adv.Finished || len(adv.Updates) != 2 {
		t.Fatalf("want 2 updates, got %+v", adv)
	}
	if st.Match("upper-r2-m1").Home() != "p1" {
		t.Fatal("winner should feed upper-r2-m1 home")
	}
	if st.Match("lower-r2-m1").Away() != "p4" {
		t.Fatal("loser should feed lower-r2-m1 away")
	}
}

func TestDoubleEliminationRejectsCounts(t *testing.T) {
	_, err := DoubleElimination{}.Plan(seeded(6), Options{})
	var ce *CountError
	if !errors.As(err, &ce) || ce.Rule != PowerOfTwo {
		t.Fatalf("6 participants: %v", err)
	}
	if _, err := (DoubleElimination{}).Plan(seeded(2), Options{}); !errors.As(err, &ce) || ce.Rule != AtLeast || ce.Need != 4 {
		t.Fatalf("2 participants: %v", err)
	}
}

func TestDoubleEliminationFullRun(t *testing.T) {
	for _, n := range []int{4, 8, 16, 64} {
		f := DoubleElimination{}
		st := planOne(t, f, seeded(n), Options{})
		reports := playAll(t, f, st, higherSeedWins)
		if !st.Finished || st.Winner != "p1" {
			t.Fatalf("n=%d: finished=%v winner=%q", n, st.Finished, st.Winner)
		}
		if reports != len(st.Matches) {
			t.Fatalf("n=%d: %d reports for %d matches", n, reports, len(st.Matches))
		}
		// Everyone but the champion lost exactly twice, except upper-bracket
		// starters eliminated... count eliminations: each non-champion has
		// exactly one loss in an elimination match.
		elim := map[string]int{}
		for _, m := range st.Matches {
			if m.Elimination {
				elim[m.Loser()]++
			}
		}
		for id, c := range elim {
			if c != 1 {
				t.Fatalf("n=%d: %s eliminated %d times", n, id, c)
			}
		}
		if len(elim) != n-2 {
			t.Fatalf("n=%d: %d eliminated in the lower bracket, want %d", n, len(elim), n-2)
		}
	}
}
