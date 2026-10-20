package brackets

// GroupStage splits participants into groups and schedules them.
//
// With MatchSize 2 (default) every group plays a round robin (circle
// method, Legs times, home/away swapped on even legs); match IDs are
// "g<group>-r<round>-m<number>". With MatchSize > 2 every group plays
// LobbyRounds free-for-all lobbies of MatchSize participants, rotating the
// group order by one position each lobby round; match IDs are
// "g<group>-r<round>-lobby<number>". Match numbers count across groups
// within a round.
//
// Groups are filled with Options.Seeding (Snake by default) using
// ceil(n/GroupSize) groups, so the last groups may be smaller. With
// Options.Strict the participant count must be a multiple of GroupSize.
// Lobbies smaller than MatchSize are scheduled (and flagged Partial) unless
// partial lobbies are disabled; lobbies of fewer than 2 are never scheduled.
//
// The stage finishes when every match is resolved. Advance also returns the
// overall standings; use [GroupStandings] for per-group tables.
type GroupStage struct{}

func (GroupStage) Key() string { return KeyGroupStage }

func (GroupStage) Validate(opts Options) error {
	_, err := groupOptions(KeyGroupStage, opts, 2, 1)
	return err
}

func (GroupStage) Plan(participants []Participant, opts Options) ([]Stage, error) {
	ids, err := seededIDs(participants)
	if err != nil {
		return nil, err
	}
	o, err := groupOptions(KeyGroupStage, opts, 2, 1)
	if err != nil {
		return nil, err
	}
	if len(ids) < 2 {
		return nil, &CountError{Format: KeyGroupStage, Rule: AtLeast, Need: 2, Got: len(ids)}
	}
	st, err := planGroups(KeyGroupStage, o, ids, true)
	if err != nil {
		return nil, err
	}
	if st.Name == "" {
		st.Name = "Group Stage"
	}
	return []Stage{st}, nil
}

func (GroupStage) Advance(st *Stage, _ *Match) (*Advance, error) {
	return &Advance{Standings: Standings(st), Finished: allResolved(st.Matches)}, nil
}

// RoundRobin plans a single group in which every participant meets every
// other participant Legs times (home and away swap on even legs). With an
// odd count one participant rests each round. Match IDs are "gA-r<round>-m<n>".
// Standings are kept on the stage; when every match is resolved the stage
// finishes and the top of the standings is the winner.
type RoundRobin struct{}

func (RoundRobin) Key() string { return KeyRoundRobin }

func (RoundRobin) Validate(opts Options) error {
	if opts.Legs < 0 {
		return &OptionError{Format: KeyRoundRobin, Field: "Legs", Reason: "must not be negative"}
	}
	return nil
}

func (f RoundRobin) Plan(participants []Participant, opts Options) ([]Stage, error) {
	ids, err := seededIDs(participants)
	if err != nil {
		return nil, err
	}
	if err := f.Validate(opts); err != nil {
		return nil, err
	}
	if len(ids) < 2 {
		return nil, &CountError{Format: KeyRoundRobin, Rule: AtLeast, Need: 2, Got: len(ids)}
	}
	o := opts
	o.Legs = max(o.Legs, 1)
	o.GroupSize = len(ids)
	o.MatchSize = 2
	o.LobbyRounds = 1
	o.Seeding = Sequential
	o.Strict = false
	o.Partial = PartialAllow
	st, err := planGroups(KeyRoundRobin, o, ids, true)
	if err != nil {
		return nil, err
	}
	st.Type = "round_robin"
	if st.Name == "" {
		st.Name = "Round Robin"
	}
	st.Standings = newTable(ids, nil, nil).standings(identity(len(ids)))
	return []Stage{st}, nil
}

func (RoundRobin) Advance(st *Stage, _ *Match) (*Advance, error) {
	out := &Advance{Standings: Standings(st), Finished: allResolved(st.Matches)}
	if out.Finished {
		out.Winner = top(out.Standings)
	}
	return out, nil
}

// FFALeague plans a free-for-all lobby league: participants are split into
// groups (default 4) that play LobbyRounds (default 4) lobbies of MatchSize
// (default 4) participants, as in battle-royale or arena-shooter leagues.
// Results are reported per slot (Placement, Points or Score). The stage
// keeps standings and finishes, with the top of the standings as winner,
// once every lobby is resolved.
type FFALeague struct{}

func (FFALeague) Key() string { return KeyFFALeague }

func (FFALeague) Validate(opts Options) error {
	_, err := groupOptions(KeyFFALeague, ffaDefaults(opts), 4, 4)
	return err
}

func ffaDefaults(o Options) Options {
	if o.GroupSize == 0 {
		o.GroupSize = 4
	}
	if o.MatchSize == 0 {
		o.MatchSize = 4
	}
	if o.LobbyRounds == 0 {
		o.LobbyRounds = 4
	}
	return o
}

func (FFALeague) Plan(participants []Participant, opts Options) ([]Stage, error) {
	ids, err := seededIDs(participants)
	if err != nil {
		return nil, err
	}
	o, err := groupOptions(KeyFFALeague, ffaDefaults(opts), 4, 4)
	if err != nil {
		return nil, err
	}
	if len(ids) < 2 {
		return nil, &CountError{Format: KeyFFALeague, Rule: AtLeast, Need: 2, Got: len(ids)}
	}
	st, err := planGroups(KeyFFALeague, o, ids, true)
	if err != nil {
		return nil, err
	}
	st.Type = "ffa_league"
	if st.Name == "" {
		st.Name = "FFA League"
	}
	st.Standings = newTable(ids, nil, nil).standings(identity(len(ids)))
	return []Stage{st}, nil
}

func (FFALeague) Advance(st *Stage, _ *Match) (*Advance, error) {
	out := &Advance{Standings: Standings(st), Finished: allResolved(st.Matches)}
	if out.Finished {
		out.Winner = top(out.Standings)
	}
	return out, nil
}

func identity(n int) []int32 {
	p := make([]int32, n)
	for i := range p {
		p[i] = int32(i)
	}
	return p
}

// groupOptions applies group-stage defaults and validates the result.
func groupOptions(format string, o Options, defMatchSize, defLobbyRounds int) (Options, error) {
	bad := func(field, reason string) (Options, error) {
		return o, &OptionError{Format: format, Field: field, Reason: reason}
	}
	if o.GroupSize == 0 {
		o.GroupSize = 4
	}
	if o.MatchSize == 0 {
		o.MatchSize = defMatchSize
	}
	if o.Legs == 0 {
		o.Legs = 1
	}
	if o.LobbyRounds == 0 {
		o.LobbyRounds = defLobbyRounds
	}
	switch {
	case o.GroupSize < 2:
		return bad("GroupSize", "must be at least 2")
	case o.MatchSize < 2:
		return bad("MatchSize", "must be at least 2")
	case o.Legs < 1:
		return bad("Legs", "must be at least 1")
	case o.LobbyRounds < 1:
		return bad("LobbyRounds", "must be at least 1")
	case o.Advances < 0 || o.AdvancesPerGroup < 0:
		return bad("Advances", "must not be negative")
	case o.Seeding > Sequential:
		return bad("Seeding", "unknown seeding")
	case o.Partial > PartialDeny:
		return bad("Partial", "unknown partial policy")
	}
	return o, nil
}

func allowPartial(o Options) bool {
	switch o.Partial {
	case PartialAllow:
		return true
	case PartialDeny:
		return false
	}
	return !o.Strict
}

// planGroups builds a group stage over refs (participant IDs when real is
// true, placeholders otherwise). Options must be normalized.
func planGroups(format string, o Options, refs []string, real bool) (Stage, error) {
	n := len(refs)
	if n < 2 {
		return Stage{}, &CountError{Format: format, Rule: AtLeast, Need: 2, Got: n}
	}
	if o.Strict && n%o.GroupSize != 0 {
		return Stage{}, &CountError{Format: format, Rule: MultipleOf, Need: o.GroupSize, Got: n}
	}
	groups := assignGroups(refs, o.GroupSize, o.Seeding)
	var matches []Match
	if o.MatchSize == 2 {
		matches = roundRobinMatches(o, groups, real)
	} else {
		matches = lobbyMatches(o, groups, real)
	}
	if len(matches) == 0 {
		return Stage{}, &CountError{Format: format, Rule: NoPlayableMatches, Got: n}
	}
	return Stage{
		Name:         o.Name,
		Format:       format,
		Type:         "group",
		Options:      o,
		Participants: refs,
		Groups:       groups,
		Matches:      matches,
		Rounds:       maxRound(matches),
	}, nil
}

func assignGroups(refs []string, groupSize int, seeding Seeding) []Group {
	count := max((len(refs)+groupSize-1)/groupSize, 1)
	groups := make([]Group, count)
	for g := range groups {
		groups[g].Name = GroupName(g)
	}
	if seeding == Sequential {
		for g := range groups {
			lo := g * groupSize
			hi := min(lo+groupSize, len(refs))
			if lo < hi {
				groups[g].Participants = refs[lo:hi:hi]
			}
		}
		return groups
	}
	sizes := make([]int, count)
	for i := range refs {
		sizes[snakePos(i, count)]++
	}
	backing := make([]string, len(refs))
	off := 0
	for g := range groups {
		groups[g].Participants = backing[off : off : off+sizes[g]]
		off += sizes[g]
	}
	for i, id := range refs {
		g := snakePos(i, count)
		groups[g].Participants = append(groups[g].Participants, id)
	}
	return groups
}

func snakePos(i, count int) int {
	pos := i % count
	if (i/count)%2 == 1 {
		pos = count - 1 - pos
	}
	return pos
}

func makeSlot(ref string, real bool) Slot {
	if real {
		return Slot{ID: ref}
	}
	return Slot{Placeholder: ref}
}

// circle returns the circle-method schedule of k participants (indices),
// as rounds of (home, away) index pairs. With odd k a dummy (-1) rests.
func circle(k int) [][][2]int {
	if k < 2 {
		return nil
	}
	teams := make([]int, k, k+1)
	for i := range teams {
		teams[i] = i
	}
	if k%2 == 1 {
		teams = append(teams, -1)
	}
	t := len(teams)
	rounds := make([][][2]int, t-1)
	pairs := make([][2]int, 0, (t-1)*(t/2))
	for r := range rounds {
		start := len(pairs)
		for i := 0; i < t/2; i++ {
			home, away := teams[i], teams[t-1-i]
			if home < 0 || away < 0 {
				continue
			}
			pairs = append(pairs, [2]int{home, away})
		}
		rounds[r] = pairs[start:len(pairs):len(pairs)]
		// Rotate every position but the first: the last moves to index 1.
		last := teams[t-1]
		copy(teams[2:], teams[1:t-1])
		teams[1] = last
	}
	return rounds
}

func roundRobinMatches(o Options, groups []Group, real bool) []Match {
	total := 0
	schedules := make([][][][2]int, len(groups))
	for g := range groups {
		schedules[g] = circle(len(groups[g].Participants))
		for _, r := range schedules[g] {
			total += len(r) * o.Legs
		}
	}
	matches := make([]Match, 0, total)
	slots := make([]Slot, 2*total)
	ib := newIDBuilder(total, 12)
	var perRound []int
	for g, group := range groups {
		sched := schedules[g]
		for leg := range o.Legs {
			for ri, pairs := range sched {
				round := leg*len(sched) + ri + 1
				for len(perRound) < round {
					perRound = append(perRound, 0)
				}
				for _, pr := range pairs {
					home, away := group.Participants[pr[0]], group.Participants[pr[1]]
					if leg%2 == 1 {
						home, away = away, home
					}
					perRound[round-1]++
					num := perRound[round-1]
					ib.s("g").s(group.Name).s("-r").n(round).s("-m").n(num).end()
					i := len(matches)
					s := slots[2*i : 2*i+2 : 2*i+2]
					s[0], s[1] = makeSlot(home, real), makeSlot(away, real)
					matches = append(matches, Match{
						Round: round, Number: num, Slots: s, Size: 2,
						Group: group.Name, GroupIndex: g, Leg: leg + 1,
					})
				}
			}
		}
	}
	for i, id := range ib.build() {
		matches[i].ID = id
	}
	return matches
}

func lobbyMatches(o Options, groups []Group, real bool) []Match {
	partial := allowPartial(o)
	n, widest := 0, 0
	for _, g := range groups {
		n += len(g.Participants)
		widest = max(widest, len(g.Participants))
	}
	perRound := (n + o.MatchSize - 1) / o.MatchSize
	matches := make([]Match, 0, perRound*o.LobbyRounds)
	slots := make([]Slot, 0, n*o.LobbyRounds)
	ib := newIDBuilder(perRound*o.LobbyRounds, 16)
	rotated := make([]string, widest)
	for lr := 1; lr <= o.LobbyRounds; lr++ {
		num := 0
		for g, group := range groups {
			k := len(group.Participants)
			if k == 0 {
				continue
			}
			rotated = rotated[:k]
			for i, id := range group.Participants {
				rotated[(i+lr-1)%k] = id
			}
			for c := 0; c < k; c += o.MatchSize {
				end := min(c+o.MatchSize, k)
				size := end - c
				if size < 2 || (size < o.MatchSize && !partial) {
					continue
				}
				num++
				ib.s("g").s(group.Name).s("-r").n(lr).s("-lobby").n(num).end()
				start := len(slots)
				for _, ref := range rotated[c:end] {
					slots = append(slots, makeSlot(ref, real))
				}
				matches = append(matches, Match{
					Round: lr, Number: num, Size: o.MatchSize,
					Group: group.Name, GroupIndex: g, LobbyRound: lr,
					Partial: size < o.MatchSize,
					Slots:   slots[start:len(slots):len(slots)],
				})
			}
		}
	}
	// slots may have been reallocated while growing: re-slice by offsets.
	off := 0
	for i := range matches {
		k := len(matches[i].Slots)
		matches[i].Slots = slots[off : off+k : off+k]
		off += k
	}
	for i, id := range ib.build() {
		matches[i].ID = id
	}
	return matches
}
