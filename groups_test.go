package brackets

import (
	"errors"
	"testing"
)

func TestRoundRobinAllPairings(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 8, 9} {
		for legs := 1; legs <= 2; legs++ {
			st := planOne(t, RoundRobin{}, seeded(n), Options{Legs: legs})
			if want := n * (n - 1) / 2 * legs; len(st.Matches) != want {
				t.Fatalf("n=%d legs=%d: %d matches, want %d", n, legs, len(st.Matches), want)
			}
			met := map[[2]string]int{}
			perRound := map[int]map[string]bool{}
			for _, m := range st.Matches {
				a, b := m.Home(), m.Away()
				if a > b {
					a, b = b, a
				}
				met[[2]string{a, b}]++
				if perRound[m.Round] == nil {
					perRound[m.Round] = map[string]bool{}
				}
				for _, id := range []string{m.Home(), m.Away()} {
					if perRound[m.Round][id] {
						t.Fatalf("n=%d: %s plays twice in round %d", n, id, m.Round)
					}
					perRound[m.Round][id] = true
				}
			}
			for pair, c := range met {
				if c != legs {
					t.Fatalf("n=%d: %v met %d times", n, pair, c)
				}
			}
		}
	}
}

func TestRoundRobinStandingsAndWinner(t *testing.T) {
	f := RoundRobin{}
	st := planOne(t, f, seeded(4), Options{})
	if len(st.Standings) != 4 {
		t.Fatal("initial standings")
	}
	playAll(t, f, st, higherSeedWins)
	if !st.Finished || st.Winner != "p1" || st.Standings[0].Wins != 3 || st.Standings[3].Losses != 3 {
		t.Fatalf("standings %+v", st.Standings)
	}
}

func TestGroupStageHeadToHead(t *testing.T) {
	st := planOne(t, GroupStage{}, unseeded(4), Options{GroupSize: 4, Seeding: Sequential})
	if len(st.Matches) != 6 {
		t.Fatalf("want 6 matches, got %d", len(st.Matches))
	}
	for _, m := range st.Matches {
		if m.Home() == "" || m.Away() == "" || m.Size != 2 {
			t.Fatalf("head-to-head match %+v", m)
		}
	}
}

func TestGroupStageLobbies(t *testing.T) {
	st := planOne(t, GroupStage{}, unseeded(8), Options{GroupSize: 4, MatchSize: 4, LobbyRounds: 2})
	if len(st.Matches) != 4 {
		t.Fatalf("want 4 lobbies, got %d", len(st.Matches))
	}
	for _, m := range st.Matches {
		if len(m.Slots) != 4 || m.LobbyRound == 0 {
			t.Fatalf("lobby %+v", m)
		}
	}
	// Snake seeding: A gets 1,4,5,8 and B gets 2,3,6,7.
	if g := st.Groups[0].Participants; g[0] != "p1" || g[1] != "p4" || g[2] != "p5" || g[3] != "p8" {
		t.Fatalf("group A %v", g)
	}
}

func TestGroupStagePartialLobby(t *testing.T) {
	st := planOne(t, GroupStage{}, unseeded(6), Options{GroupSize: 4, MatchSize: 4})
	if len(st.Matches) != 2 || !st.Matches[1].Partial {
		t.Fatalf("want a partial second lobby, got %+v", st.Matches)
	}
	_, err := GroupStage{}.Plan(unseeded(5), Options{GroupSize: 4, MatchSize: 4, Strict: true})
	var ce *CountError
	if !errors.As(err, &ce) || ce.Rule != MultipleOf || ce.Need != 4 {
		t.Fatalf("strict: %v", err)
	}
	_, err = GroupStage{}.Plan(unseeded(4), Options{GroupSize: 3, MatchSize: 4, Partial: PartialDeny})
	if !errors.As(err, &ce) || ce.Rule != NoPlayableMatches {
		t.Fatalf("no playable: %v", err)
	}
}

func TestGroupStageOptionsValidation(t *testing.T) {
	for _, o := range []Options{{GroupSize: 1}, {MatchSize: 1}, {Legs: -1}, {LobbyRounds: -2}, {Advances: -1}, {Seeding: 9}, {Partial: 9}} {
		if err := (GroupStage{}).Validate(o); !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("%+v: %v", o, err)
		}
	}
}

func TestFFALeague(t *testing.T) {
	f := FFALeague{}
	st := planOne(t, f, seeded(8), Options{LobbyRounds: 2})
	if len(st.Matches) != 4 || st.Type != "ffa_league" {
		t.Fatalf("want 4 lobbies, got %d", len(st.Matches))
	}
	// p1 always places first, the others by seed.
	playAll(t, f, st, func(m *Match) Result {
		pl := make([]int, len(m.Slots))
		pts := make([]int, len(m.Slots))
		for i := range m.Slots {
			pl[i] = i + 1
			pts[i] = 10 - i
		}
		return Result{Placements: pl, Points: pts}
	})
	if !st.Finished || st.Winner == "" {
		t.Fatalf("league should finish: %+v", st.Standings)
	}
}

func TestStandingsTieBreakers(t *testing.T) {
	st := &Stage{
		Participants: []string{"a", "b", "c", "d", "e"},
		Matches: []Match{
			// a beats b 3-0, c beats d 1-0, e beats a 1-0: a, c, e 1-? ...
			{Status: Finished, Winner: "a", Slots: []Slot{{ID: "a", Score: 3}, {ID: "b"}}},
			{Status: Finished, Winner: "c", Slots: []Slot{{ID: "c", Score: 1}, {ID: "d"}}},
			{Status: Finished, Winner: "", Slots: []Slot{{ID: "e", Score: 2}, {ID: "b", Score: 2}}},
			{Status: Pending, Winner: "d", Slots: []Slot{{ID: "d"}, {ID: "e"}}},
			{Status: Finished, Winner: "x", Slots: []Slot{{ID: "x", Score: 5}, {ID: "d"}}},
		},
	}
	got := Standings(st)
	order := []string{}
	for _, s := range got {
		order = append(order, s.ID)
	}
	// a (1-0, +3), c (1-0, +1), x (1-0, +5 but... )
	want := []string{"x", "a", "c", "e", "b", "d"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order %v, want %v", order, want)
		}
	}
	if got[0].Order != 5 || got[3].Draws != 1 || got[5].Losses != 2 || got[0].Rank != 1 {
		t.Fatalf("rows %+v", got)
	}
}

func TestGroupStandings(t *testing.T) {
	f := GroupStage{}
	st := planOne(t, f, seeded(8), Options{GroupSize: 4})
	playAll(t, f, st, higherSeedWins)
	tables := GroupStandings(st)
	if len(tables) != 2 || tables[0][0].ID != "p1" || tables[1][0].ID != "p2" || tables[0][0].Rank != 1 {
		t.Fatalf("tables %+v", tables)
	}
}
