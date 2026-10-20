package brackets

import (
	"math/rand/v2"
	"reflect"
	"strconv"
	"testing"
)

// bitPicker turns fuzz bytes into a deterministic stream of choices.
type bitPicker struct {
	r *rand.Rand
}

func newPicker(seed uint64) *bitPicker { return &bitPicker{r: rand.New(rand.NewPCG(seed, 7))} }
func (b *bitPicker) intn(n int) int    { return b.r.IntN(n) }

// checkOncePerRound fails if a participant occupies two slots of the same
// round (or a slot and a bye).
func checkOncePerRound(t *testing.T, st *Stage) {
	t.Helper()
	seen := map[int]map[string]bool{}
	mark := func(round int, id string) {
		if id == "" {
			return
		}
		if seen[round] == nil {
			seen[round] = map[string]bool{}
		}
		if seen[round][id] {
			t.Fatalf("%s appears twice in round %d", id, round)
		}
		seen[round][id] = true
	}
	for _, m := range st.Matches {
		for _, s := range m.Slots {
			mark(m.Round, s.Ref())
		}
	}
	for _, b := range st.Byes {
		mark(b.Round, b.Participant)
	}
}

func FuzzSingleElimination(f *testing.F) {
	for _, n := range []uint16{2, 3, 5, 8, 13, 64, 100, 257} {
		f.Add(n, uint64(n))
	}
	f.Fuzz(func(t *testing.T, n16 uint16, seed uint64) {
		n := int(n16%600) + 2
		pick := newPicker(seed)
		ps := seeded(n)
		pick.r.Shuffle(len(ps), func(i, j int) { ps[i], ps[j] = ps[j], ps[i] })
		fm := SingleElimination{}
		st := planOne(t, fm, ps, Options{})
		again := planOne(t, fm, ps, Options{})
		if !reflect.DeepEqual(st, again) {
			t.Fatal("plan is not deterministic")
		}
		size := st.Size
		if !isPowerOfTwo(size) || size < n || size/2 >= n || len(st.Matches) != size-1 {
			t.Fatalf("n=%d size=%d matches=%d", n, size, len(st.Matches))
		}
		// Round 1 holds every participant exactly once; byes go to the top
		// size-n seeds and nothing is phantom.
		inRoundOne := map[string]int{}
		byes := map[string]bool{}
		for _, m := range st.Matches {
			if m.Phantom {
				t.Fatalf("unexpected phantom %s", m.ID)
			}
			if m.Round != 1 {
				continue
			}
			for _, s := range m.Slots {
				if s.ID != "" {
					inRoundOne[s.ID]++
				}
			}
			if m.Bye {
				byes[m.Winner] = true
			}
		}
		if len(inRoundOne) != n {
			t.Fatalf("round 1 has %d participants, want %d", len(inRoundOne), n)
		}
		for id, c := range inRoundOne {
			if c != 1 {
				t.Fatalf("%s appears %d times in round 1", id, c)
			}
		}
		if len(byes) != size-n {
			t.Fatalf("%d byes, want %d", len(byes), size-n)
		}
		for i := 1; i <= size-n; i++ {
			if !byes["p"+strconv.Itoa(i)] {
				t.Fatalf("seed %d has no bye", i)
			}
		}
		checkOncePerRound(t, st)
		playAll(t, fm, st, func(m *Match) Result { return Result{Winner: m.Slots[pick.intn(2)].ID} })
		if !st.Finished || st.Winner == "" {
			t.Fatal("bracket did not finish")
		}
		checkOncePerRound(t, st)
		losses := map[string]int{}
		for _, m := range st.Matches {
			if m.Status != Finished {
				t.Fatalf("%s not finished", m.ID)
			}
			if l := m.Loser(); l != "" {
				losses[l]++
			}
		}
		if len(losses) != n-1 || losses[st.Winner] != 0 {
			t.Fatalf("%d participants lost, want %d", len(losses), n-1)
		}
		for id, c := range losses {
			if c != 1 {
				t.Fatalf("%s lost %d times in single elimination", id, c)
			}
		}
	})
}

func FuzzDoubleElimination(f *testing.F) {
	for k := uint8(0); k < 6; k++ {
		f.Add(k, uint64(k))
	}
	f.Fuzz(func(t *testing.T, k uint8, seed uint64) {
		n := 4 << (k % 7)
		pick := newPicker(seed)
		fm := DoubleElimination{}
		st := planOne(t, fm, seeded(n), Options{})
		checkOncePerRound(t, st)
		reports := playAll(t, fm, st, func(m *Match) Result { return Result{Winner: m.Slots[pick.intn(2)].ID} })
		if !st.Finished || reports != len(st.Matches) {
			t.Fatalf("n=%d finished=%v reports=%d matches=%d", n, st.Finished, reports, len(st.Matches))
		}
		checkOncePerRound(t, st)
		eliminated := map[string]int{}
		gf := st.Match("grand-final")
		for _, m := range st.Matches {
			if m.Elimination {
				eliminated[m.Loser()]++
			}
		}
		for id, c := range eliminated {
			if c != 1 || id == st.Winner || id == gf.Loser() {
				t.Fatalf("%s eliminated %d times (winner %s)", id, c, st.Winner)
			}
		}
		if len(eliminated) != n-2 {
			t.Fatalf("%d eliminated, want %d", len(eliminated), n-2)
		}
	})
}

// rematchFree reports whether pool can be perfectly paired without any
// pair in met (exhaustive; small pools only).
func rematchFree(pool []string, met map[[2]string]bool) bool {
	if len(pool) == 0 {
		return true
	}
	a := pool[0]
	for i := 1; i < len(pool); i++ {
		b := pool[i]
		if met[[2]string{a, b}] {
			continue
		}
		rest := make([]string, 0, len(pool)-2)
		rest = append(rest, pool[1:i]...)
		rest = append(rest, pool[i+1:]...)
		if rematchFree(rest, met) {
			return true
		}
	}
	return false
}

func FuzzSwiss(f *testing.F) {
	f.Add(uint8(5), uint8(3), false, false, uint64(1))
	f.Add(uint8(8), uint8(7), true, false, uint64(2))
	f.Add(uint8(11), uint8(9), false, true, uint64(3))
	f.Add(uint8(32), uint8(5), true, true, uint64(4))
	f.Fuzz(func(t *testing.T, n8, rounds8 uint8, greedy, shuffle bool, seed uint64) {
		n := int(n8%40) + 2
		rounds := int(rounds8)%max(n-1, 1) + 1
		pairing := AvoidRematches
		if greedy {
			pairing = Greedy
		}
		opts := Options{Rounds: rounds, Pairing: pairing, Shuffle: shuffle, Seed: seed}
		fm := Swiss{}
		run := func() *Stage {
			pick := newPicker(seed)
			st := planOne(t, fm, seeded(n), opts)
			playAll(t, fm, st, func(m *Match) Result {
				switch pick.intn(5) {
				case 0:
					return Result{Scores: []int{1, 1}} // draw
				case 1, 2:
					return Result{Winner: m.Slots[0].ID, Scores: []int{2, pick.intn(2)}}
				}
				return Result{Winner: m.Slots[1].ID, Scores: []int{pick.intn(2), 2}}
			})
			return st
		}
		st := run()
		if !reflect.DeepEqual(st, run()) {
			t.Fatal("swiss is not deterministic for the same inputs")
		}
		if !st.Finished || maxRound(st.Matches) != rounds {
			t.Fatalf("n=%d: finished=%v rounds=%d want %d", n, st.Finished, maxRound(st.Matches), rounds)
		}
		checkOncePerRound(t, st)
		// Every participant plays or rests exactly once per round.
		for r := 1; r <= rounds; r++ {
			count := 0
			byes := 0
			for _, m := range st.Matches {
				if m.Round == r {
					count += 2
				}
			}
			for _, b := range st.Byes {
				if b.Round == r {
					byes++
				}
			}
			if count+byes != n || byes != n%2 {
				t.Fatalf("round %d: %d playing + %d byes for %d participants", r, count, byes, n)
			}
		}
		// Byes: nobody gets a second bye while someone has none.
		byeCount := map[string]int{}
		for _, b := range st.Byes {
			byeCount[b.Participant]++
			if byeCount[b.Participant] > 1 && len(byeCount) < n {
				t.Fatalf("%s got a second bye in round %d before everyone had one", b.Participant, b.Round)
			}
		}
		// Rematches: with AvoidRematches a rematch only happens when the
		// round's pool has no rematch-free pairing.
		if pairing == AvoidRematches && n <= 12 {
			met := map[[2]string]bool{}
			for r := 1; r <= rounds; r++ {
				var pool []string
				rematch := false
				for _, m := range st.Matches {
					if m.Round != r {
						continue
					}
					pool = append(pool, m.Home(), m.Away())
					if met[[2]string{m.Home(), m.Away()}] {
						rematch = true
					}
				}
				if rematch && rematchFree(pool, met) {
					t.Fatalf("round %d has an avoidable rematch", r)
				}
				for _, m := range st.Matches {
					if m.Round == r {
						met[[2]string{m.Home(), m.Away()}] = true
						met[[2]string{m.Away(), m.Home()}] = true
					}
				}
			}
		}
		// Standings conservation: every decisive match is one win and one
		// loss, every bye one win, every draw two draws.
		wins, losses, draws := 0, 0, 0
		for _, s := range st.Standings {
			wins += s.Wins
			losses += s.Losses
			draws += s.Draws
		}
		decisive, drawn := 0, 0
		for _, m := range st.Matches {
			if m.Winner == "" {
				drawn++
			} else {
				decisive++
			}
		}
		if wins != decisive+len(st.Byes) || losses != decisive || draws != 2*drawn {
			t.Fatalf("standings do not add up: W%d L%d D%d vs %d decisive %d draws %d byes", wins, losses, draws, decisive, drawn, len(st.Byes))
		}
	})
}

func FuzzGroupStage(f *testing.F) {
	f.Add(uint8(8), uint8(4), uint8(2), uint8(1), uint8(1), false, false)
	f.Add(uint8(13), uint8(4), uint8(4), uint8(1), uint8(3), true, false)
	f.Add(uint8(17), uint8(5), uint8(3), uint8(2), uint8(2), false, true)
	f.Fuzz(func(t *testing.T, n8, gs8, ms8, legs8, lr8 uint8, sequential, strict bool) {
		n := int(n8%48) + 2
		o := Options{
			GroupSize:   int(gs8%7) + 2,
			MatchSize:   int(ms8%5) + 2,
			Legs:        int(legs8%2) + 1,
			LobbyRounds: int(lr8%3) + 1,
			Strict:      strict,
		}
		if sequential {
			o.Seeding = Sequential
		}
		stages, err := GroupStage{}.Plan(seeded(n), o)
		if err != nil {
			if strict && n%o.GroupSize != 0 {
				return
			}
			if _, ok := err.(*CountError); ok {
				return
			}
			t.Fatalf("plan: %v", err)
		}
		st := &stages[0]
		group := map[string]int{}
		minSize, maxSize := n, 0
		for g, grp := range st.Groups {
			minSize = min(minSize, len(grp.Participants))
			maxSize = max(maxSize, len(grp.Participants))
			for _, id := range grp.Participants {
				if _, dup := group[id]; dup {
					t.Fatalf("%s in two groups", id)
				}
				group[id] = g
			}
		}
		if len(group) != n || maxSize > o.GroupSize {
			t.Fatalf("groups cover %d of %d (max size %d)", len(group), n, maxSize)
		}
		if !sequential && maxSize-minSize > 1 {
			t.Fatalf("snake groups unbalanced: %d..%d", minSize, maxSize)
		}
		checkOncePerRound(t, st)
		met := map[[2]string]int{}
		for _, m := range st.Matches {
			for _, s := range m.Slots {
				if group[s.ID] != m.GroupIndex {
					t.Fatalf("%s plays outside its group in %s", s.ID, m.ID)
				}
			}
			if o.MatchSize == 2 {
				a, b := m.Home(), m.Away()
				if a > b {
					a, b = b, a
				}
				met[[2]string{a, b}]++
			} else if len(m.Slots) > o.MatchSize || len(m.Slots) < 2 || m.Partial != (len(m.Slots) < o.MatchSize) {
				t.Fatalf("lobby %s has %d slots (size %d, partial %v)", m.ID, len(m.Slots), o.MatchSize, m.Partial)
			}
		}
		if o.MatchSize == 2 {
			for _, grp := range st.Groups {
				for i, a := range grp.Participants {
					for _, b := range grp.Participants[i+1:] {
						x, y := a, b
						if x > y {
							x, y = y, x
						}
						if met[[2]string{x, y}] != o.Legs {
							t.Fatalf("%s-%s met %d times, want %d", x, y, met[[2]string{x, y}], o.Legs)
						}
					}
				}
			}
		}
	})
}
