package brackets

// Result is the outcome of a match, reported through [Report].
type Result struct {
	// Winner is the winning participant ID. Leave it empty for a draw
	// (head-to-head formats without elimination) or when lobby placements
	// decide the result.
	Winner string
	// Scores, Points and Placements are per slot, in slot order. Any of
	// them may be nil. A non-nil Points marks every slot as having points.
	Scores     []int
	Points     []int
	Placements []int
}

// Apply applies the outcome of [Format.Advance] to the stage: fills slots,
// appends new matches, records byes, standings, scoreboard and winner.
func (s *Stage) Apply(a *Advance) {
	if a == nil {
		return
	}
	for _, u := range a.Updates {
		m := &s.Matches[u.Index]
		if u.Slot < len(m.Slots) {
			m.Slots[u.Slot].ID = u.Participant
			m.Slots[u.Slot].Placeholder = ""
		}
	}
	s.Matches = append(s.Matches, a.NewMatches...)
	s.Byes = append(s.Byes, a.NewByes...)
	if a.Standings != nil {
		s.Standings = a.Standings
	}
	if a.CurrentRound > 0 {
		s.CurrentRound = a.CurrentRound
	}
	if a.Scoreboard != nil {
		s.Scoreboard = a.Scoreboard
	}
	if a.Winner != "" {
		s.Winner = a.Winner
	}
	if a.Finished {
		s.Finished = true
	}
}

// Report records r on the match with the given ID, advances the stage with
// f and applies the outcome. It validates the result: the match must be
// pending or live with every slot filled, the winner must be one of its
// participants, and elimination matches need a winner.
func Report(f Format, st *Stage, matchID string, r Result) (*Advance, error) {
	idx := linkIndex(st, Link{Match: matchID, Index: -1})
	if idx < 0 {
		return nil, &LinkError{From: matchID, Target: matchID}
	}
	return ReportAt(f, st, idx, r)
}

// ReportAt is [Report] for the match at index i of st.Matches; it skips
// the lookup by ID.
func ReportAt(f Format, st *Stage, i int, r Result) (*Advance, error) {
	if i < 0 || i >= len(st.Matches) {
		return nil, &LinkError{From: "#" + itoa(i), Target: "#" + itoa(i)}
	}
	matchID := st.Matches[i].ID
	m := &st.Matches[i]
	invalid := func(reason string) error { return &resultError{match: matchID, reason: reason} }
	if m.Status.Resolved() {
		return nil, invalid("match is already " + m.Status.String())
	}
	for _, s := range m.Slots {
		if s.ID == "" {
			return nil, invalid("match has unresolved slots")
		}
	}
	if r.Winner != "" && !m.Has(r.Winner) {
		return nil, invalid("winner " + quote(r.Winner) + " is not in the match")
	}
	if r.Winner == "" && m.Elimination {
		return nil, invalid("elimination matches need a winner")
	}
	if len(r.Scores) > len(m.Slots) || len(r.Points) > len(m.Slots) || len(r.Placements) > len(m.Slots) {
		return nil, invalid("more results than slots")
	}
	for i := range m.Slots {
		s := &m.Slots[i]
		if i < len(r.Scores) {
			s.Score = r.Scores[i]
		}
		if i < len(r.Points) {
			s.Points, s.HasPoints = r.Points[i], true
		}
		if i < len(r.Placements) {
			s.Placement = r.Placements[i]
		}
	}
	m.Winner = r.Winner
	m.Status = Finished
	adv, err := f.Advance(st, m)
	if err != nil {
		return nil, err
	}
	st.Apply(adv)
	return adv, nil
}

type resultError struct{ match, reason string }

func (e *resultError) Error() string { return "brackets: match " + quote(e.match) + ": " + e.reason }
func (e *resultError) Unwrap() error { return ErrInvalidResult }
