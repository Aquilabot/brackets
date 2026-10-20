package brackets

import "math/rand/v2"

// Swiss plans a Swiss-system stage: a fixed number of rounds in which
// participants with similar records meet, without eliminations.
//
// Round 1 pairs adjacent seeds (1v2, 3v4, ...) unless Options.Shuffle is
// set. Every later round is generated when all matches of the current round
// are resolved: participants are ordered by the current standings (see
// [Standing]) and paired with Options.Pairing (AvoidRematches by default).
//
// Byes: with an odd number of participants the lowest-ranked participant
// that has not had a bye yet sits out and is credited a win (if everyone
// already had one, the lowest-ranked participant gets another).
//
// Match IDs are "swiss-r<round>-m<number>". The stage finishes when the
// last round is resolved, or earlier if no pairing is possible; the winner
// is the top of the standings.
type Swiss struct{}

func (Swiss) Key() string { return KeySwiss }

func (Swiss) Validate(opts Options) error {
	if opts.Rounds < 0 {
		return &OptionError{Format: KeySwiss, Field: "Rounds", Reason: "must not be negative"}
	}
	if opts.Pairing > Greedy {
		return &OptionError{Format: KeySwiss, Field: "Pairing", Reason: "unknown pairing"}
	}
	return nil
}

func (f Swiss) Plan(participants []Participant, opts Options) ([]Stage, error) {
	ids, err := seededIDs(participants)
	if err != nil {
		return nil, err
	}
	if len(ids) < 2 {
		return nil, &CountError{Format: KeySwiss, Rule: AtLeast, Need: 2, Got: len(ids)}
	}
	if err := f.Validate(opts); err != nil {
		return nil, err
	}
	if opts.Rounds == 0 {
		opts.Rounds = DefaultSwissRounds(len(ids))
	}
	t := newTable(ids, nil, nil)
	perm := t.sorted()
	pairs, bye := pairRound(t, perm, nil, nil, opts, 1)
	var byes []Bye
	if bye >= 0 {
		byes = []Bye{{Round: 1, Participant: t.rows[bye].ID}}
		t.rows[bye].Wins++
		t.rows[bye].Byes++
		t.sortPerm(perm)
	}
	name := opts.Name
	if name == "" {
		name = "Swiss"
	}
	return []Stage{{
		Name:         name,
		Format:       KeySwiss,
		Type:         "swiss",
		Options:      opts,
		Participants: ids,
		Matches:      pairMatches(t, 1, pairs),
		Rounds:       opts.Rounds,
		CurrentRound: 1,
		Byes:         byes,
		Standings:    t.standings(perm),
	}}, nil
}

func (f Swiss) Advance(st *Stage, _ *Match) (*Advance, error) {
	t := NewTally(st.Participants, st.Byes)
	for i := range st.Matches {
		t.Add(&st.Matches[i])
	}
	return f.AdvanceTally(st, t)
}

// AdvanceTally is [Swiss.Advance] for callers that keep matches in their
// own storage: the stage's matches are read from t (fed with every match
// of the stage) instead of st.Matches. st supplies the configuration:
// Participants, Rounds, CurrentRound and Options. AdvanceTally finalizes t.
func (Swiss) AdvanceTally(st *Stage, t *Tally) (*Advance, error) {
	cur := t.MaxRound()
	if cur == 0 {
		cur = max(st.CurrentRound, 1)
	}
	if !t.RoundResolved(cur) {
		return &Advance{Standings: t.Standings()}, nil
	}
	tb := t.t
	perm := tb.sorted()
	total := st.Rounds
	if total == 0 {
		total = st.Options.Rounds
	}
	if total == 0 {
		total = DefaultSwissRounds(t.participants)
	}
	if cur >= total {
		out := &Advance{Standings: tb.standings(perm), Finished: true}
		out.Winner = top(out.Standings)
		return out, nil
	}

	hist := t.history()
	hadBye := make([]bool, len(tb.rows))
	for _, b := range t.byes {
		if i, ok := tb.find(b.Participant); ok {
			hadBye[i] = true
		}
	}
	next := cur + 1
	pairs, bye := pairRound(tb, perm, hist, hadBye, st.Options, next)
	out := &Advance{}
	if bye >= 0 {
		out.NewByes = []Bye{{Round: next, Participant: tb.rows[bye].ID}}
		tb.rows[bye].Wins++
		tb.rows[bye].Byes++
		tb.sortPerm(perm)
	}
	out.Standings = tb.standings(perm)
	if len(pairs) == 0 {
		out.Winner = top(out.Standings)
		out.Finished = true
		return out, nil
	}
	out.CurrentRound = next
	out.NewMatches = pairMatches(tb, next, pairs)
	return out, nil
}

func top(s []Standing) string {
	if len(s) == 0 {
		return ""
	}
	return s[0].ID
}

func pairMatches(t *table, round int, pairs [][2]int32) []Match {
	ib := newIDBuilder(len(pairs), 16)
	for i := range pairs {
		ib.s("swiss-r").n(round).s("-m").n(i + 1).end()
	}
	idList := ib.build()
	matches := make([]Match, len(pairs))
	slots := make([]Slot, 2*len(pairs))
	for i, p := range pairs {
		s := slots[2*i : 2*i+2 : 2*i+2]
		s[0], s[1] = slotFor(t.rows[p[0]].ID), slotFor(t.rows[p[1]].ID)
		matches[i] = Match{ID: idList[i], Round: round, Number: i + 1, Slots: s, Size: 2}
	}
	return matches
}

// slotFor builds a slot from a participant ID or placeholder reference.
func slotFor(ref string) Slot {
	if IsPlaceholder(ref) {
		return Slot{Placeholder: ref}
	}
	return Slot{ID: ref}
}

// history is the set of pairs that already met, as a compact adjacency
// list (CSR) over table rows. Swiss histories are short (one opponent per
// round), so a linear scan beats hashing.
type history struct {
	start []int32
	adj   []int32
}

func (h *history) met(a, b int32) bool {
	if h == nil {
		return false
	}
	for _, o := range h.adj[h.start[a]:h.start[a+1]] {
		if o == b {
			return true
		}
	}
	return false
}

// pairRound pairs the rows in perm (ranking order). It returns the pairs
// (higher-ranked participant first) and the row receiving a bye, or -1.
func pairRound(t *table, perm []int32, h *history, hadBye []bool, opts Options, round int) ([][2]int32, int32) {
	order := make([]int32, len(perm))
	copy(order, perm)
	if opts.Shuffle {
		src := opts.Rand
		if src == nil {
			src = rand.NewPCG(opts.Seed, uint64(round))
		}
		shuffleTies(t, order, rand.New(src))
	}
	bye := int32(-1)
	if len(order)%2 == 1 {
		at := len(order) - 1
		for i := len(order) - 1; i >= 0; i-- {
			if hadBye == nil || !hadBye[order[i]] {
				at = i
				break
			}
		}
		bye = order[at]
		order = append(order[:at], order[at+1:]...)
	}
	p := newPairer(order, h)
	if opts.Pairing == AvoidRematches && h != nil {
		if pairs, ok := p.search(); ok {
			return pairs, bye
		}
		p = newPairer(order, h)
	}
	return p.greedy(), bye
}

// shuffleTies shuffles runs of rows whose records are identical (every
// tie-breaker except seed order).
func shuffleTies(t *table, order []int32, r *rand.Rand) {
	same := func(a, b int32) bool {
		x, y := &t.rows[a], &t.rows[b]
		return x.Wins == y.Wins && x.Losses == y.Losses && x.Diff() == y.Diff() && x.PointsFor == y.PointsFor
	}
	for i := 0; i < len(order); {
		j := i + 1
		for j < len(order) && same(order[i], order[j]) {
			j++
		}
		run := order[i:j]
		r.Shuffle(len(run), func(a, b int) { run[a], run[b] = run[b], run[a] })
		i = j
	}
}

// searchBudget bounds the backtracking search (opponent candidates tried
// per round). Past it, pairing falls back to Greedy.
const searchBudget = 1 << 20

// pairer keeps the unpaired participants in a doubly linked list (indices
// into order, sentinel at len(order)) so removal and restoration are O(1),
// as in dancing links.
type pairer struct {
	order      []int32
	next, prev []int32
	h          *history
	pairs      [][2]int32
	budget     int
}

func newPairer(order []int32, h *history) *pairer {
	n := int32(len(order))
	p := &pairer{order: order, next: make([]int32, n+1), prev: make([]int32, n+1), h: h,
		pairs: make([][2]int32, 0, n/2), budget: searchBudget}
	for i := int32(0); i <= n; i++ {
		p.next[i] = (i + 1) % (n + 1)
		p.prev[i] = (i + n) % (n + 1)
	}
	return p
}

func (p *pairer) sentinel() int32 { return int32(len(p.order)) }

func (p *pairer) remove(i int32) {
	p.next[p.prev[i]] = p.next[i]
	p.prev[p.next[i]] = p.prev[i]
}

func (p *pairer) restore(i int32) {
	p.next[p.prev[i]] = i
	p.prev[p.next[i]] = i
}

func (p *pairer) met(i, j int32) bool { return p.h.met(p.order[i], p.order[j]) }

// greedy pairs the first unpaired participant with the first unpaired
// opponent it has not met, or with the next one if it met them all.
func (p *pairer) greedy() [][2]int32 {
	s := p.sentinel()
	for a := p.next[s]; a != s; a = p.next[s] {
		p.remove(a)
		first := p.next[s]
		if first == s {
			break
		}
		b := first
		for q := first; q != s; q = p.next[q] {
			if !p.met(a, q) {
				b = q
				break
			}
		}
		p.remove(b)
		p.pairs = append(p.pairs, [2]int32{p.order[a], p.order[b]})
	}
	return p.pairs
}

// search is a depth-first search over the greedy order: its first complete
// solution is the greedy pairing whenever that pairing has no rematch.
func (p *pairer) search() ([][2]int32, bool) {
	ok := p.dfs()
	return p.pairs, ok
}

func (p *pairer) dfs() bool {
	s := p.sentinel()
	a := p.next[s]
	if a == s {
		return true
	}
	p.remove(a)
	for q := p.next[s]; q != s; q = p.next[q] {
		if p.budget--; p.budget < 0 {
			return false
		}
		if p.met(a, q) {
			continue
		}
		p.remove(q)
		p.pairs = append(p.pairs, [2]int32{p.order[a], p.order[q]})
		if p.dfs() {
			return true
		}
		if p.budget < 0 {
			return false
		}
		p.pairs = p.pairs[:len(p.pairs)-1]
		p.restore(q)
	}
	p.restore(a)
	return false
}
