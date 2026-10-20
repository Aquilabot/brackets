package brackets

// Tally accumulates a stage's matches one at a time: standings, the pairs
// that already met, and round completion. It lets callers that keep
// matches in their own storage advance a stage without first copying every
// match into Stage.Matches: build the tally, feed each stored match through
// a reused Match value with Add, then read Standings or call
// [Swiss.AdvanceTally].
//
// The result is identical to the stage-based functions over the same
// matches in the same order ([Standings], [Swiss.Advance]).
type Tally struct {
	t            *table
	tb           table
	participants int
	byes         []Bye
	met          []string // flattened pairs of refs from two-slot matches
	rounds       []roundCount
	roundsBuf    [16]roundCount
	maxRound     int
	open         bool
}

type roundCount struct{ total, open int }

// NewTally starts a tally over the stage's participants (in seed order)
// and the byes granted so far.
func NewTally(participants []string, byes []Bye) *Tally {
	t := &Tally{participants: len(participants), byes: byes}
	t.tb.init(participants, byes)
	t.t = &t.tb
	t.rounds = t.roundsBuf[:0]
	return t
}

// Grow reserves room for n more matches. It is optional and only saves
// reallocations when the number of matches is known up front.
func (t *Tally) Grow(n int) {
	if free := cap(t.met) - len(t.met); free < 2*n {
		met := make([]string, len(t.met), len(t.met)+2*n)
		copy(met, t.met)
		t.met = met
	}
}

// Add records one match. The tally keeps no reference to m or its slots,
// so callers can reuse the same Match value for every call.
func (t *Tally) Add(m *Match) {
	resolved := m.Status.Resolved()
	if !resolved {
		t.open = true
	}
	if r := m.Round; r > 0 {
		if r > t.maxRound {
			t.maxRound = r
		}
		for len(t.rounds) < r {
			t.rounds = append(t.rounds, roundCount{})
		}
		t.rounds[r-1].total++
		if !resolved {
			t.rounds[r-1].open++
		}
	}
	var refs [2]string
	k := 0
	for si := range m.Slots {
		if ref := m.Slots[si].Ref(); ref != "" {
			if k < 2 {
				refs[k] = ref
			}
			k++
		}
	}
	if k == 2 {
		t.met = append(t.met, refs[0], refs[1])
	}
	if m.Status == Finished {
		t.t.addMatch(m)
	}
}

// MaxRound returns the highest round number added (0 if none).
func (t *Tally) MaxRound() int { return t.maxRound }

// RoundResolved reports whether round r has matches and all are resolved.
func (t *Tally) RoundResolved(r int) bool {
	return r >= 1 && r <= len(t.rounds) && t.rounds[r-1].total > 0 && t.rounds[r-1].open == 0
}

// AllResolved reports whether no added match is pending or live.
func (t *Tally) AllResolved() bool { return !t.open }

// Standings returns the ranked standings (see [Standing]). It finalizes
// the tally: do not call Add, Standings or AdvanceTally on it afterwards.
func (t *Tally) Standings() []Standing { return t.t.ranked() }

// history resolves the recorded pairs to table rows.
func (t *Tally) history() *history {
	n := len(t.t.rows)
	h := &history{start: make([]int32, n+1)}
	deg := make([]int32, n)
	rows := make([]int32, 0, len(t.met))
	for i := 0; i+1 < len(t.met); i += 2 {
		a, okA := t.t.find(t.met[i])
		b, okB := t.t.find(t.met[i+1])
		if !okA || !okB {
			continue
		}
		rows = append(rows, a, b)
		deg[a]++
		deg[b]++
	}
	for i := 0; i < n; i++ {
		h.start[i+1] = h.start[i] + deg[i]
	}
	h.adj = make([]int32, h.start[n])
	copy(deg, h.start[:n])
	for i := 0; i+1 < len(rows); i += 2 {
		a, b := rows[i], rows[i+1]
		h.adj[deg[a]] = b
		deg[a]++
		h.adj[deg[b]] = a
		deg[b]++
	}
	return h
}
