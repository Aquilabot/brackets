package brackets

// DoubleElimination plans an upper/lower bracket in the style used by many
// esports main events: the top half of the seeds starts in the upper
// bracket, the bottom half starts in the lower bracket, and a participant is
// eliminated after a loss in the lower bracket.
//
// The participant count must be a power of two and at least 4. With n
// participants the upper bracket has n/2 participants and log2(n/2) rounds;
// the lower bracket has 2*log2(n/2) rounds that alternate between
// "lower winners meet upper losers" (even rounds) and "lower winners meet
// each other" (odd rounds). Upper-bracket losers of upper round r drop into
// lower round 2r. The grand final has no game advantage for the upper
// bracket winner.
//
// Series hints: upper matches and lower rounds 2+ are best-of-3, lower
// round 1 is best-of-1, the grand final is best-of-5.
//
// Match IDs are "upper-r<round>-m<n>", "lower-r<round>-m<n>" and
// "grand-final". Match.Round is a schedule column: upper round r is column
// 2r-1, lower round r is column r, the grand final is column 2*upperRounds+1.
// The stage finishes when the grand final is reported.
type DoubleElimination struct{}

func (DoubleElimination) Key() string { return KeyDoubleElimination }

func (DoubleElimination) Validate(Options) error { return nil }

func (DoubleElimination) Plan(participants []Participant, opts Options) ([]Stage, error) {
	ids, err := seededIDs(participants)
	if err != nil {
		return nil, err
	}
	n := len(ids)
	if n < 4 {
		return nil, &CountError{Format: KeyDoubleElimination, Rule: AtLeast, Need: 4, Got: n}
	}
	if !isPowerOfTwo(n) {
		return nil, &CountError{Format: KeyDoubleElimination, Rule: PowerOfTwo, Got: n}
	}
	upperSize := n / 2
	upperRounds := log2(upperSize)
	lowerRounds := 2 * upperRounds
	upperIDs, lowerIDs := ids[:upperSize], ids[upperSize:]

	lowerCount := func(r int) int {
		if r%2 == 0 {
			return upperSize >> (r / 2)
		}
		return upperSize >> ((r + 1) / 2)
	}
	upperTotal := upperSize - 1
	lowerTotal := 0
	for r := 1; r <= lowerRounds; r++ {
		lowerTotal += lowerCount(r)
	}
	total := upperTotal + lowerTotal + 1

	// Index helpers.
	upperStart := func(r int) int { return upperSize - (upperSize >> (r - 1)) } // r is 1-based
	lowerStarts := make([]int, lowerRounds+2)
	lowerStarts[1] = upperTotal
	for r := 1; r <= lowerRounds; r++ {
		lowerStarts[r+1] = lowerStarts[r] + lowerCount(r)
	}
	gf := total - 1

	ib := newIDBuilder(total, 14)
	for r := 1; r <= upperRounds; r++ {
		for m := 1; m <= upperSize>>r; m++ {
			ib.s("upper-r").n(r).s("-m").n(m).end()
		}
	}
	for r := 1; r <= lowerRounds; r++ {
		for m := 1; m <= lowerCount(r); m++ {
			ib.s("lower-r").n(r).s("-m").n(m).end()
		}
	}
	ib.s("grand-final").end()
	idList := ib.build()

	matches := make([]Match, total)
	slots := make([]Slot, 2*total)
	links := make([]Link, 0, 2*total)
	from := make([]string, 0, 2*total)
	link := func(target, slot int) *Link {
		links = append(links, Link{Match: idList[target], Slot: slot, Index: target})
		return &links[len(links)-1]
	}
	feeders := func(a, b int) []string {
		from = append(from, idList[a], idList[b])
		return from[len(from)-2 : len(from) : len(from)]
	}

	upperOrder := SeedOrder(upperSize)
	for r := 1; r <= upperRounds; r++ {
		count := upperSize >> r
		for m := 1; m <= count; m++ {
			i := upperStart(r) + m - 1
			match := &matches[i]
			*match = Match{
				ID: idList[i], Round: 2*r - 1, Number: m, Size: 2,
				Slots:   slots[2*i : 2*i+2 : 2*i+2],
				Bracket: UpperBracket, BracketRound: r, BestOf: 3,
			}
			if r == 1 {
				// The upper bracket is a standard bracket of upperSize slots.
				match.Slots[0].ID = upperIDs[upperOrder[2*(m-1)]-1]
				match.Slots[1].ID = upperIDs[upperOrder[2*(m-1)+1]-1]
			} else {
				prev := upperStart(r-1) + 2*(m-1)
				match.From = feeders(prev, prev+1)
			}
			if r < upperRounds {
				match.WinnerTo = link(upperStart(r+1)+(m+1)/2-1, slotOf(m))
			} else {
				match.WinnerTo = link(gf, 0)
			}
			lm := m
			if r == upperRounds {
				lm = 1
			}
			match.LoserTo = link(lowerStarts[2*r]+lm-1, 1)
		}
	}

	lowerOrder := SeedOrder(upperSize)
	for r := 1; r <= lowerRounds; r++ {
		count := lowerCount(r)
		for m := 1; m <= count; m++ {
			i := lowerStarts[r] + m - 1
			match := &matches[i]
			bo := 3
			if r == 1 {
				bo = 1
			}
			*match = Match{
				ID: idList[i], Round: r, Number: m, Size: 2,
				Slots:   slots[2*i : 2*i+2 : 2*i+2],
				Bracket: LowerBracket, BracketRound: r, BestOf: bo, Elimination: true,
			}
			switch {
			case r == 1:
				match.Slots[0].ID = lowerIDs[lowerOrder[2*(m-1)]-1]
				match.Slots[1].ID = lowerIDs[lowerOrder[2*(m-1)+1]-1]
			case r%2 == 0:
				// Winner of the lower match above, then the upper loser.
				match.From = feeders(lowerStarts[r-1]+m-1, upperStart(r/2)+m-1)
			default:
				prev := lowerStarts[r-1] + 2*(m-1)
				match.From = feeders(prev, prev+1)
			}
			switch {
			case r == lowerRounds:
				match.WinnerTo = link(gf, 1)
			case r%2 == 0:
				match.WinnerTo = link(lowerStarts[r+1]+(m+1)/2-1, slotOf(m))
			default:
				match.WinnerTo = link(lowerStarts[r+1]+m-1, 0)
			}
		}
	}

	matches[gf] = Match{
		ID: idList[gf], Round: 2*upperRounds + 1, Number: 1, Size: 2,
		Slots:   slots[2*gf : 2*gf+2 : 2*gf+2],
		Bracket: GrandFinal, BracketRound: 1, BestOf: 5,
		From: feeders(upperStart(upperRounds), lowerStarts[lowerRounds]),
	}

	name := opts.Name
	if name == "" {
		name = "Double Elimination"
	}
	return []Stage{{
		Name:         name,
		Format:       KeyDoubleElimination,
		Type:         "double_elimination",
		Options:      opts,
		Participants: ids,
		Matches:      matches,
		Rounds:       2*upperRounds + 1,
		Size:         n,
		UpperSize:    upperSize,
		UpperRounds:  upperRounds,
		LowerRounds:  lowerRounds,
	}}, nil
}

// Advance sends the winner along WinnerTo and the loser along LoserTo.
// Targets are resolved by match ID, so the links stored on the matches are
// the source of truth.
func (f DoubleElimination) Advance(st *Stage, finished *Match) (*Advance, error) {
	out := &Advance{}
	if finished == nil {
		return out, nil
	}
	mv := f.Moves(finished)
	if mv.Final {
		out.Winner = mv.Winner
		out.Finished = true
		return out, nil
	}
	if mv.WinnerTo != nil {
		u, err := linkUpdate(st, finished.ID, *mv.WinnerTo, mv.Winner)
		if err != nil {
			return nil, err
		}
		out.Updates = append(out.Updates, u)
	}
	if mv.LoserTo != nil {
		u, err := linkUpdate(st, finished.ID, *mv.LoserTo, mv.Loser)
		if err != nil {
			return nil, err
		}
		out.Updates = append(out.Updates, u)
	}
	return out, nil
}

// Moves describes where the participants of a finished match go next.
type Moves struct {
	Winner, Loser     string
	LoserSlot         int   // index of the loser in the match slots, or -1
	WinnerTo, LoserTo *Link // nil when that participant goes nowhere
	Final             bool  // the match decided the stage
}

// Moves returns the outcome of a finished double-elimination match from
// the match alone: the winner follows WinnerTo, the loser (if any) follows
// LoserTo, and a decided grand final ends the stage. A match without a
// winner moves nobody. Callers that store matches themselves can resolve
// the target IDs in their own storage instead of building a Stage.
func (DoubleElimination) Moves(finished *Match) Moves {
	mv := Moves{Winner: finished.Winner, LoserSlot: -1}
	if finished.Winner == "" {
		return mv
	}
	if finished.Bracket == GrandFinal {
		mv.Final = true
		return mv
	}
	mv.WinnerTo = finished.WinnerTo
	if i := finished.LoserSlot(); i >= 0 {
		mv.LoserSlot, mv.Loser = i, finished.Slots[i].ID
		mv.LoserTo = finished.LoserTo
	}
	return mv
}

func linkUpdate(st *Stage, from string, l Link, participant string) (SlotUpdate, error) {
	i := linkIndex(st, l)
	if i < 0 {
		return SlotUpdate{}, &LinkError{From: from, Target: l.Match}
	}
	return SlotUpdate{Index: i, Match: l.Match, Slot: l.Slot, Participant: participant}, nil
}

func slotOf(matchNumber int) int {
	if matchNumber%2 == 1 {
		return 0
	}
	return 1
}
