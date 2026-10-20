package brackets

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestSwissFiveParticipantsNextRound(t *testing.T) {
	f := Swiss{}
	st := planOne(t, f, seeded(5), Options{Rounds: 3})
	if len(st.Matches) != 2 || len(st.Byes) != 1 || st.Byes[0].Participant != "p5" {
		t.Fatalf("round 1: %d matches, byes %+v", len(st.Matches), st.Byes)
	}
	if st.Matches[0].ID != "swiss-r1-m1" || st.Matches[0].Home() != "p1" || st.Matches[0].Away() != "p2" {
		t.Fatalf("round 1 pairs adjacent seeds, got %+v", st.Matches[0])
	}
	if _, err := Report(f, st, "swiss-r1-m1", Result{Winner: "p1"}); err != nil {
		t.Fatal(err)
	}
	if st.CurrentRound != 1 || len(st.Matches) != 2 {
		t.Fatal("round 2 must wait for the whole round")
	}
	adv, err := Report(f, st, "swiss-r1-m2", Result{Winner: "p3"})
	if err != nil {
		t.Fatal(err)
	}
	if adv.Finished || len(adv.NewMatches) != 2 || adv.CurrentRound != 2 {
		t.Fatalf("want 2 new matches in round 2, got %+v", adv)
	}
	if len(st.Byes) != 2 || st.Byes[1].Participant == "p5" {
		t.Fatalf("second bye must go to someone who has not had one: %+v", st.Byes)
	}
	playAll(t, f, st, homeWins)
	if !st.Finished || st.Winner == "" {
		t.Fatalf("stage should finish with a winner: %+v", st.Standings)
	}
	if got := maxRound(st.Matches); got != 3 {
		t.Fatalf("want 3 rounds, got %d", got)
	}
}

// Four participants at 0-0 where 1 already played 2: pairing must avoid
// the rematch.
func TestSwissAvoidsRematch(t *testing.T) {
	for _, pairing := range []Pairing{AvoidRematches, Greedy} {
		st := &Stage{
			Participants: []string{"p1", "p2", "p3", "p4"},
			Rounds:       3,
			Options:      Options{Pairing: pairing},
			Matches: []Match{
				{ID: "x", Round: 1, Number: 1, Status: Finished, Slots: []Slot{{ID: "p1"}, {ID: "p2"}}},
				{ID: "y", Round: 1, Number: 2, Status: Finished, Slots: []Slot{{ID: "p3"}, {ID: "p4"}}},
			},
		}
		adv, err := Swiss{}.Advance(st, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range adv.NewMatches {
			if m.Has("p1") && m.Has("p2") || m.Has("p3") && m.Has("p4") {
				t.Fatalf("%v: rematch %s vs %s", pairing, m.Home(), m.Away())
			}
		}
	}
}

// Greedy pairs top-down: p1 takes p2, leaving p3 and p4, who already met.
// AvoidRematches backtracks to p1-p3, p2-p4.
func TestSwissAvoidRematchesBeatsGreedy(t *testing.T) {
	base := Stage{
		Participants: []string{"p1", "p2", "p3", "p4"},
		Rounds:       3,
		Matches: []Match{
			// A cancelled match still counts as "already met" for pairing.
			{ID: "x", Round: 1, Number: 1, Status: Cancelled, Slots: []Slot{{ID: "p3"}, {ID: "p4"}}},
		},
	}
	pairsOf := func(p Pairing) [][2]string {
		st := base
		st.Options.Pairing = p
		adv, err := Swiss{}.Advance(&st, nil)
		if err != nil {
			t.Fatal(err)
		}
		var out [][2]string
		for _, m := range adv.NewMatches {
			out = append(out, [2]string{m.Home(), m.Away()})
		}
		return out
	}
	if got := pairsOf(Greedy); !slices.Equal(got, [][2]string{{"p1", "p2"}, {"p3", "p4"}}) {
		t.Fatalf("greedy = %v", got)
	}
	if got := pairsOf(AvoidRematches); !slices.Equal(got, [][2]string{{"p1", "p3"}, {"p2", "p4"}}) {
		t.Fatalf("avoid rematches = %v", got)
	}
}

func TestSwissShuffleDeterministic(t *testing.T) {
	ps := seeded(16)
	pairing := func(opts Options) []string {
		st := planOne(t, Swiss{}, ps, opts)
		var ids []string
		for _, m := range st.Matches {
			ids = append(ids, m.Home()+"-"+m.Away())
		}
		return ids
	}
	a := pairing(Options{Shuffle: true, Seed: 42})
	b := pairing(Options{Shuffle: true, Seed: 42})
	c := pairing(Options{Shuffle: true, Seed: 43})
	d := pairing(Options{Shuffle: true, Rand: rand.NewPCG(42, 1)})
	if !slices.Equal(a, b) {
		t.Fatal("same seed must give the same pairing")
	}
	if slices.Equal(a, c) {
		t.Fatal("different seeds should give different pairings")
	}
	if !slices.Equal(a, d) {
		t.Fatal("Seed s must equal an explicit PCG(s, round) source")
	}
	if slices.Equal(a, pairing(Options{})) {
		t.Fatal("shuffled round 1 should differ from seed order")
	}
}

func TestSwissWinnerAndStandings(t *testing.T) {
	f := Swiss{}
	st := planOne(t, f, seeded(8), Options{})
	if st.Rounds != 3 {
		t.Fatalf("default rounds for 8 = %d", st.Rounds)
	}
	playAll(t, f, st, higherSeedWins)
	if !st.Finished || st.Winner != "p1" {
		t.Fatalf("p1 wins every match, got winner %q", st.Winner)
	}
	if st.Standings[0].Wins != 3 || st.Standings[0].Losses != 0 {
		t.Fatalf("top standing %+v", st.Standings[0])
	}
}

func TestSwissValidate(t *testing.T) {
	if err := (Swiss{}).Validate(Options{Rounds: -1}); err == nil {
		t.Fatal("negative rounds should fail")
	}
	if _, err := (Swiss{}).Plan(seeded(1), Options{}); err == nil {
		t.Fatal("one participant should fail")
	}
	if DefaultSwissRounds(2) != 1 || DefaultSwissRounds(5) != 3 || DefaultSwissRounds(64) != 6 || DefaultSwissRounds(65) != 7 {
		t.Fatal("DefaultSwissRounds")
	}
}
