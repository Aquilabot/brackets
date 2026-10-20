package brackets

// SingleElimination plans one knockout bracket.
//
// The bracket size is the next power of two. Participants are placed with
// the standard seed order ([SeedOrder]), so the empty slots (byes) always
// face the top seeds: with n participants and size S, seeds 1..S-n advance
// to round 2 without playing. Bye matches are Finished with Bye set at
// planning time and their winners are already placed in round 2.
//
// Match IDs are "r<round>-m<number>". The stage finishes when every match is
// resolved; the winner of the final is reported as the stage winner.
type SingleElimination struct{}

func (SingleElimination) Key() string { return KeySingleElimination }

func (SingleElimination) Validate(Options) error { return nil }

func (SingleElimination) Plan(participants []Participant, opts Options) ([]Stage, error) {
	ids, err := seededIDs(participants)
	if err != nil {
		return nil, err
	}
	n := len(ids)
	if n < 2 {
		return nil, &CountError{Format: KeySingleElimination, Rule: AtLeast, Need: 2, Got: n}
	}
	size := NextPowerOfTwo(n)
	rounds := log2(size)
	order := SeedOrder(size)

	total := size - 1
	matches := make([]Match, total)
	slots := make([]Slot, 2*total)
	from := make([]string, 2*(total-size/2))
	links := make([]Link, total-1)

	ib := newIDBuilder(total, 8)
	for r := range rounds {
		count := size >> (r + 1)
		for m := range count {
			ib.s("r").n(r + 1).s("-m").n(m + 1).end()
		}
	}
	idList := ib.build()

	// roundStart(r) is the index of the first match of 0-based round r.
	roundStart := func(r int) int { return size - (size >> r) }

	for r := range rounds {
		count := size >> (r + 1)
		base := roundStart(r)
		for m := range count {
			i := base + m
			match := &matches[i]
			match.ID = idList[i]
			match.Round = r + 1
			match.Number = m + 1
			match.Size = 2
			match.Elimination = true
			match.Slots = slots[2*i : 2*i+2 : 2*i+2]
			if r == 0 {
				if s := order[2*m] - 1; s < n {
					match.Slots[0].ID = ids[s]
				}
				if s := order[2*m+1] - 1; s < n {
					match.Slots[1].ID = ids[s]
				}
			} else {
				f := 2 * (i - size/2)
				prev := roundStart(r-1) + 2*m
				from[f], from[f+1] = idList[prev], idList[prev+1]
				match.From = from[f : f+2 : f+2]
			}
			if r+1 < rounds {
				next := roundStart(r+1) + m/2
				links[i] = Link{Match: idList[next], Slot: m % 2, Index: next}
				match.WinnerTo = &links[i]
			}
		}
	}

	// Resolve byes and phantom matches round by round. A match is resolved
	// at planning time only when both feeders already are.
	for r := range rounds {
		count := size >> (r + 1)
		base := roundStart(r)
		for m := range count {
			match := &matches[base+m]
			if r > 0 {
				prev := roundStart(r-1) + 2*m
				if matches[prev].Status == Pending || matches[prev+1].Status == Pending {
					continue
				}
			}
			home, away := match.Slots[0].ID, match.Slots[1].ID
			switch {
			case home == "" && away == "":
				match.Status = Cancelled
				match.Phantom = true
			case home == "" || away == "":
				winner := home
				if winner == "" {
					winner = away
				}
				match.Status = Finished
				match.Winner = winner
				match.Bye = true
				if r+1 < rounds {
					next := &matches[roundStart(r+1)+m/2]
					next.Slots[m%2].ID = winner
				}
			}
		}
	}

	name := opts.Name
	if name == "" {
		name = "Bracket"
	}
	return []Stage{{
		Name:         name,
		Format:       KeySingleElimination,
		Type:         "bracket",
		Options:      opts,
		Participants: ids,
		Matches:      matches,
		Rounds:       rounds,
		Size:         size,
		SeedOrder:    order,
	}}, nil
}

func (SingleElimination) Advance(st *Stage, finished *Match) (*Advance, error) {
	out := &Advance{}
	if finished == nil {
		return out, nil
	}
	total := st.Rounds
	if total == 0 {
		total = maxRound(st.Matches)
	}
	if total == 0 {
		return nil, errNoRounds
	}
	if finished.Round >= total {
		out.Finished = allResolved(st.Matches)
		if out.Finished {
			out.Winner = finished.Winner
		}
		return out, nil
	}
	if finished.Winner == "" {
		out.Finished = allResolved(st.Matches)
		return out, nil
	}
	var idx, slot int
	if l := finished.WinnerTo; l != nil {
		// Planned stages carry the link: O(1) with a valid index hint.
		idx, slot = linkIndex(st, *l), l.Slot
		if idx < 0 {
			return nil, &LinkError{From: finished.ID, Target: l.Match}
		}
	} else {
		// Without links, the parent is the first match at (round+1, ceil(n/2)).
		var nextRound, nextNumber int
		nextRound, nextNumber, slot = SingleElimination{}.Parent(finished.Round, finished.Number)
		idx = findRoundMatch(st.Matches, nextRound, nextNumber)
		if idx < 0 {
			return nil, &LinkError{From: finished.ID, Round: nextRound, Number: nextNumber}
		}
	}
	out.Updates = []SlotUpdate{{Index: idx, Match: st.Matches[idx].ID, Slot: slot, Participant: finished.Winner}}
	out.Finished = allResolved(st.Matches)
	return out, nil
}

// Parent returns where the winner of match (round, number) plays next:
// the first match at (round+1, ceil(number/2)), home slot for odd numbers
// and away slot for even ones. Callers that store matches themselves can
// use it to advance a bracket without building a Stage.
func (SingleElimination) Parent(round, number int) (nextRound, nextNumber, slot int) {
	slot = 1
	if number%2 == 1 {
		slot = 0
	}
	return round + 1, (number + 1) / 2, slot
}

var errNoRounds = wrapStage("stage has no round count and no matches to infer it from")

type stageErr string

func (e stageErr) Error() string { return "brackets: " + string(e) }
func (e stageErr) Unwrap() error { return ErrInvalidStage }

func wrapStage(msg string) error { return stageErr(msg) }

func maxRound(ms []Match) int {
	m := 0
	for i := range ms {
		if ms[i].Round > m {
			m = ms[i].Round
		}
	}
	return m
}

// allResolved reports whether no match is pending or live. It scans from
// the end: later matches stay pending longest, so an unfinished stage is
// usually detected in O(1).
func allResolved(ms []Match) bool {
	for i := len(ms) - 1; i >= 0; i-- {
		if !ms[i].Status.Resolved() {
			return false
		}
	}
	return true
}

// linkIndex resolves a link to an index of st.Matches: the hint when it
// matches the ID, otherwise the first match with that ID, or -1.
func linkIndex(st *Stage, l Link) int {
	if l.Index >= 0 && l.Index < len(st.Matches) && st.Matches[l.Index].ID == l.Match {
		return l.Index
	}
	for i := range st.Matches {
		if st.Matches[i].ID == l.Match {
			return i
		}
	}
	return -1
}

// findRoundMatch returns the index of the first match at (round, number).
func findRoundMatch(ms []Match, round, number int) int {
	for i := range ms {
		if ms[i].Round == round && ms[i].Number == number {
			return i
		}
	}
	return -1
}
