package brackets

import "slices"

// Standing is one row of a standings table.
//
// Ranking (tie-breakers in order):
//
//  1. Wins, descending. A bye counts as a win.
//  2. Losses, ascending.
//  3. Score difference (PointsFor - PointsAgainst), descending.
//  4. PointsFor, descending.
//  5. Order, ascending: the participant's position in the stage's seed
//     order (participants first seen in match results come after, in order
//     of appearance). This makes the table total and deterministic.
//
// Only Finished matches count. In head-to-head matches (two slots) a
// finished match with an empty Winner is a draw for both sides, PointsFor
// is the slot's own Score and PointsAgainst the opponent's Score. In lobbies
// (any other slot count) PointsFor adds the slot's Points (or Score when no
// points were reported), Placement 1 is a win and any other reported
// placement a loss; without a placement the match Winner decides.
type Standing struct {
	Rank          int
	ID            string
	Played        int
	Wins          int
	Losses        int
	Draws         int
	Byes          int
	PointsFor     int
	PointsAgainst int
	Order         int
}

// Diff returns PointsFor - PointsAgainst.
func (s *Standing) Diff() int { return s.PointsFor - s.PointsAgainst }

// Standings computes the standings of a stage from its participants, byes
// and finished matches. See [Standing] for the tie-breakers.
func Standings(st *Stage) []Standing {
	t := NewTally(st.Participants, st.Byes)
	for i := range st.Matches {
		t.Add(&st.Matches[i])
	}
	return t.Standings()
}

// GroupStandings returns one standings table per group of the stage, in
// group order, ranked within the group.
func GroupStandings(st *Stage) [][]Standing {
	all := Standings(st)
	pos := make(map[string]int, len(all))
	for i := range all {
		pos[all[i].ID] = i
	}
	out := make([][]Standing, len(st.Groups))
	for g, group := range st.Groups {
		idx := make([]int, 0, len(group.Participants))
		for _, id := range group.Participants {
			if i, ok := pos[id]; ok {
				idx = append(idx, i)
			}
		}
		slices.Sort(idx)
		rows := make([]Standing, len(idx))
		for r, i := range idx {
			rows[r] = all[i]
			rows[r].Rank = r + 1
		}
		out[g] = rows
	}
	return out
}

// table accumulates per-participant records by interned row index.
type table struct {
	rows  []Standing
	index map[string]int32
}

func newTable(participants []string, byes []Bye, matches []Match) *table {
	t := &table{}
	t.init(participants, byes)
	for mi := range matches {
		m := &matches[mi]
		if m.Status != Finished {
			continue
		}
		t.addMatch(m)
	}
	return t
}

func (t *table) init(participants []string, byes []Bye) {
	t.rows = make([]Standing, len(participants), len(participants)+1)
	for i, id := range participants {
		t.rows[i] = Standing{ID: id, Order: i}
	}
	for _, b := range byes {
		if i, ok := t.find(b.Participant); ok {
			t.rows[i].Wins++
			t.rows[i].Byes++
		}
	}
}

// find returns the first row of id. The index map is built on first use,
// so tables that never look anything up (fresh plans) skip it.
func (t *table) find(id string) (int32, bool) {
	if t.index == nil {
		t.index = make(map[string]int32, len(t.rows))
		for i := range t.rows {
			if _, ok := t.index[t.rows[i].ID]; !ok {
				t.index[t.rows[i].ID] = int32(i)
			}
		}
	}
	i, ok := t.index[id]
	return i, ok
}

// lookup returns the row of ref, adding a row for unknown participant IDs.
// Placeholders and empty refs have no row (-1).
func (t *table) lookup(ref string) int32 {
	if i, ok := t.find(ref); ok {
		return i
	}
	if ref == "" || IsPlaceholder(ref) {
		return -1
	}
	i := int32(len(t.rows))
	t.rows = append(t.rows, Standing{ID: ref, Order: int(i)})
	t.index[ref] = i
	return i
}

func (t *table) addMatch(m *Match) {
	// Count the occupied slots and register unknown participants in slot
	// order before scoring, so new rows get a stable Order.
	occupied := 0
	first, second := -1, -1
	for si := range m.Slots {
		ref := m.Slots[si].Ref()
		if ref == "" {
			continue
		}
		t.lookup(ref)
		switch occupied {
		case 0:
			first = si
		case 1:
			second = si
		}
		occupied++
	}
	if occupied == 0 {
		return
	}
	winner := m.Winner
	if occupied == 2 {
		a, b := &m.Slots[first], &m.Slots[second]
		ra, rb := t.lookup(a.Ref()), t.lookup(b.Ref())
		t.headToHead(ra, a.Ref(), a.Score, b.Score, winner)
		t.headToHead(rb, b.Ref(), b.Score, a.Score, winner)
		return
	}
	for si := range m.Slots {
		s := &m.Slots[si]
		ref := s.Ref()
		if ref == "" {
			continue
		}
		r := t.lookup(ref)
		if r < 0 {
			continue
		}
		row := &t.rows[r]
		row.Played++
		if s.HasPoints {
			row.PointsFor += s.Points
		} else {
			row.PointsFor += s.Score
		}
		switch {
		case s.Placement == 1:
			row.Wins++
		case s.Placement > 1:
			row.Losses++
		case winner != "" && ref == winner:
			row.Wins++
		case winner != "":
			row.Losses++
		}
	}
}

func (t *table) headToHead(r int32, ref string, own, opp int, winner string) {
	if r < 0 {
		return
	}
	row := &t.rows[r]
	row.PointsFor += own
	row.PointsAgainst += opp
	switch {
	case winner == "":
		row.Draws++
	case ref == winner:
		row.Wins++
	default:
		row.Losses++
	}
	row.Played++
}

func compareStanding(a, b *Standing) int {
	if a.Wins != b.Wins {
		return b.Wins - a.Wins
	}
	if a.Losses != b.Losses {
		return a.Losses - b.Losses
	}
	if da, db := a.Diff(), b.Diff(); da != db {
		return db - da
	}
	if a.PointsFor != b.PointsFor {
		return b.PointsFor - a.PointsFor
	}
	return a.Order - b.Order
}

// sorted returns row indices in ranking order. Order is unique per row, so
// the ranking is a total order and an unstable sort is deterministic.
func (t *table) sorted() []int32 {
	perm := make([]int32, len(t.rows))
	for i := range perm {
		perm[i] = int32(i)
	}
	t.sortPerm(perm)
	return perm
}

func (t *table) sortPerm(perm []int32) {
	rows := t.rows
	slices.SortFunc(perm, func(a, b int32) int { return compareStanding(&rows[a], &rows[b]) })
}

// ranked sorts the table's own rows and returns them with ranks set. It
// sorts a permutation (4-byte moves) and then applies it in place, so no
// second row slice is allocated. The table is unusable afterwards.
func (t *table) ranked() []Standing {
	rows := t.rows
	perm := t.sorted()
	// Apply perm in place by following cycles; perm[i] < 0 marks done.
	for i := range perm {
		if perm[i] < 0 {
			continue
		}
		first := rows[i]
		j := i
		for {
			k := int(perm[j])
			perm[j] = -1
			if k == i {
				rows[j] = first
				break
			}
			rows[j] = rows[k]
			j = k
		}
	}
	for i := range rows {
		rows[i].Rank = i + 1
	}
	t.rows, t.index = nil, nil
	return rows
}

func (t *table) standings(perm []int32) []Standing {
	out := make([]Standing, len(perm))
	for i, r := range perm {
		out[i] = t.rows[r]
		out[i].Rank = i + 1
	}
	return out
}
