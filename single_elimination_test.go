package brackets

import (
	"errors"
	"slices"
	"strconv"
	"testing"
)

func TestSeedOrder(t *testing.T) {
	tests := map[int][]int{
		1:  {1},
		2:  {1, 2},
		4:  {1, 4, 2, 3},
		8:  {1, 8, 4, 5, 2, 7, 3, 6},
		16: {1, 16, 8, 9, 4, 13, 5, 12, 2, 15, 7, 10, 3, 14, 6, 11},
	}
	for size, want := range tests {
		if got := SeedOrder(size); !slices.Equal(got, want) {
			t.Errorf("SeedOrder(%d) = %v, want %v", size, got, want)
		}
	}
	// Every first-round pair sums to size+1 at every power of two.
	for size := 2; size <= 1<<12; size *= 2 {
		order := SeedOrder(size)
		for i := 0; i < size; i += 2 {
			if order[i]+order[i+1] != size+1 {
				t.Fatalf("size %d: pair %d,%d does not sum to %d", size, order[i], order[i+1], size+1)
			}
		}
	}
}

func TestSingleEliminationFourParticipants(t *testing.T) {
	st := planOne(t, SingleElimination{}, unseeded(4), Options{})
	if len(st.Matches) != 3 {
		t.Fatalf("want 3 matches, got %d", len(st.Matches))
	}
	if m := st.Matches[0]; m.Home() != "p1" || m.Away() != "p4" {
		t.Errorf("r1-m1 want p1 vs p4, got %s vs %s", m.Home(), m.Away())
	}
	if m := st.Matches[1]; m.Home() != "p2" || m.Away() != "p3" {
		t.Errorf("r1-m2 want p2 vs p3, got %s vs %s", m.Home(), m.Away())
	}
	final := st.Matches[2]
	if final.ID != "r2-m1" || final.Round != 2 || final.Number != 1 {
		t.Errorf("final = %s round %d number %d", final.ID, final.Round, final.Number)
	}
	if final.Home() != "" || final.Away() != "" {
		t.Errorf("final slots should be empty before round 1, got %q vs %q", final.Home(), final.Away())
	}
	if !slices.Equal(final.From, []string{"r1-m1", "r1-m2"}) {
		t.Errorf("final.From = %v", final.From)
	}
	if l := st.Matches[1].WinnerTo; l == nil || *l != (Link{Match: "r2-m1", Slot: 1, Index: 2}) {
		t.Errorf("r1-m2 WinnerTo = %+v", l)
	}
	if st.Size != 4 || st.Rounds != 2 {
		t.Errorf("size %d rounds %d", st.Size, st.Rounds)
	}
}

func TestSingleEliminationThreeParticipantsBye(t *testing.T) {
	st := planOne(t, SingleElimination{}, unseeded(3), Options{})
	bye := st.Matches[0]
	if bye.Status != Finished || bye.Winner != "p1" || !bye.Bye {
		t.Errorf("r1-m1 should be a finished bye for p1, got %+v", bye)
	}
	if m := st.Matches[1]; m.Home() != "p2" || m.Away() != "p3" || m.Status != Pending {
		t.Errorf("r1-m2 should be p2 vs p3 pending, got %+v", m)
	}
	if final := st.Matches[2]; final.Home() != "p1" || final.Away() != "" {
		t.Errorf("bye winner should be placed in the final, got %q vs %q", final.Home(), final.Away())
	}
}

func TestSingleEliminationFiveParticipantsCascade(t *testing.T) {
	st := planOne(t, SingleElimination{}, unseeded(5), Options{})
	if len(st.Matches) != 7 {
		t.Fatalf("want 7 matches, got %d", len(st.Matches))
	}
	byKey := map[string]*Match{}
	for i := range st.Matches {
		byKey[st.Matches[i].ID] = &st.Matches[i]
	}
	for id, winner := range map[string]string{"r1-m1": "p1", "r1-m3": "p2", "r1-m4": "p3"} {
		if m := byKey[id]; m.Status != Finished || m.Winner != winner || !m.Bye {
			t.Errorf("%s should be a bye for %s, got %+v", id, winner, m)
		}
	}
	if m := byKey["r1-m2"]; m.Home() != "p4" || m.Away() != "p5" || m.Status != Pending {
		t.Errorf("r1-m2 want p4 vs p5, got %+v", m)
	}
	if m := byKey["r2-m2"]; m.Home() != "p2" || m.Away() != "p3" || m.Status != Pending {
		t.Errorf("r2-m2 want p2 vs p3 pending, got %+v", m)
	}
	if m := byKey["r3-m1"]; m.Home() != "" || m.Away() != "" || m.Status != Pending {
		t.Errorf("final should be empty and pending, got %+v", m)
	}
}

func TestSingleEliminationByesGoToTopSeeds(t *testing.T) {
	tests := []struct{ n, size, matches, byes, roundOne int }{
		{12, 16, 15, 4, 4},
		{512, 512, 511, 0, 256},
		{513, 1024, 1023, 511, 1},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.n), func(t *testing.T) {
			st := planOne(t, SingleElimination{}, unseeded(tt.n), Options{})
			if st.Size != tt.size || len(st.Matches) != tt.matches {
				t.Fatalf("size %d matches %d", st.Size, len(st.Matches))
			}
			winners := map[string]bool{}
			pending := 0
			for _, m := range st.Matches {
				if m.Round != 1 {
					continue
				}
				if m.Bye {
					winners[m.Winner] = true
				} else if m.Status == Pending {
					pending++
				}
			}
			if len(winners) != tt.byes || pending != tt.roundOne {
				t.Fatalf("byes %d round-one games %d", len(winners), pending)
			}
			for i := 1; i <= tt.byes; i++ {
				if !winners["p"+strconv.Itoa(i)] {
					t.Fatalf("seed %d should have a bye", i)
				}
			}
		})
	}
}

func TestSingleEliminationRejectsTooFew(t *testing.T) {
	_, err := SingleElimination{}.Plan(unseeded(1), Options{})
	var ce *CountError
	if !errors.As(err, &ce) || ce.Need != 2 || ce.Got != 1 || !errors.Is(err, ErrParticipantCount) {
		t.Fatalf("want CountError, got %v", err)
	}
}

func TestSingleEliminationRunToFinish(t *testing.T) {
	f := SingleElimination{}
	st := planOne(t, f, unseeded(4), Options{})

	adv, err := Report(f, st, "r1-m1", Result{Winner: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Matches[2].Home() != "p1" || adv.Finished {
		t.Fatalf("after r1-m1: final home %q finished %v", st.Matches[2].Home(), adv.Finished)
	}
	if _, err := Report(f, st, "r1-m2", Result{Winner: "p2"}); err != nil {
		t.Fatal(err)
	}
	if st.Matches[2].Away() != "p2" || st.Finished {
		t.Fatalf("after r1-m2: final away %q finished %v", st.Matches[2].Away(), st.Finished)
	}
	adv, err = Report(f, st, "r2-m1", Result{Winner: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if !adv.Finished || st.Winner != "p1" {
		t.Fatalf("final should finish the stage with p1, got %+v", adv)
	}
}

func TestReportValidation(t *testing.T) {
	f := SingleElimination{}
	st := planOne(t, f, unseeded(4), Options{})
	cases := []struct {
		match string
		r     Result
		want  error
	}{
		{"nope", Result{Winner: "p1"}, ErrMatchNotFound},
		{"r1-m1", Result{Winner: "p2"}, ErrInvalidResult},
		{"r1-m1", Result{}, ErrInvalidResult},
		{"r2-m1", Result{Winner: "p1"}, ErrInvalidResult},
		{"r1-m1", Result{Winner: "p1", Scores: []int{1, 2, 3}}, ErrInvalidResult},
	}
	for _, c := range cases {
		if _, err := Report(f, st, c.match, c.r); !errors.Is(err, c.want) {
			t.Errorf("Report(%s, %+v) = %v, want %v", c.match, c.r, err, c.want)
		}
	}
	if _, err := Report(f, st, "r1-m1", Result{Winner: "p1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Report(f, st, "r1-m1", Result{Winner: "p1"}); !errors.Is(err, ErrInvalidResult) {
		t.Errorf("reporting twice should fail, got %v", err)
	}
}

func TestSingleEliminationAdvanceErrors(t *testing.T) {
	f := SingleElimination{}
	if _, err := f.Advance(&Stage{}, &Match{Round: 1, Winner: "a"}); !errors.Is(err, ErrInvalidStage) {
		t.Errorf("empty stage: %v", err)
	}
	st := &Stage{Rounds: 2, Matches: []Match{{ID: "r1-m1", Round: 1, Number: 1}}}
	_, err := f.Advance(st, &Match{ID: "r1-m1", Round: 1, Number: 1, Winner: "a"})
	var le *LinkError
	if !errors.As(err, &le) || le.Round != 2 || le.Number != 1 {
		t.Errorf("missing downstream: %v", err)
	}
	if out, err := f.Advance(st, nil); err != nil || out.Finished {
		t.Errorf("nil finished: %+v %v", out, err)
	}
}

func TestParticipantValidation(t *testing.T) {
	for _, ps := range [][]Participant{
		{{ID: "a"}, {ID: ""}},
		{{ID: "a"}, {ID: "a"}},
		{{ID: "a"}, {ID: "slot:x:1"}},
	} {
		if _, err := (SingleElimination{}).Plan(ps, Options{}); !errors.Is(err, ErrInvalidParticipant) {
			t.Errorf("%v: want ErrInvalidParticipant, got %v", ps, err)
		}
	}
}

func TestSortParticipants(t *testing.T) {
	in := []Participant{{"u1", 0}, {"s3", 3}, {"u2", -1}, {"s1", 1}, {"s1b", 1}}
	got := SortParticipants(in)
	want := []string{"s1", "s1b", "s3", "u1", "u2"}
	for i, p := range got {
		if p.ID != want[i] {
			t.Fatalf("got %v", got)
		}
	}
	if in[0].ID != "u1" {
		t.Fatal("input mutated")
	}
}
