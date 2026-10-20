package brackets

// MultiStage chains stages described by Options.Stages. Each stage either
// takes every participant or, when StageSpec.Source names an earlier stage,
// StageSpec.Take placeholders ("slot:<source>:<rank>", see [Placeholder])
// to be replaced by that stage's qualifiers once it finishes (see
// [FillPlaceholders] and [Qualifiers]).
//
// Stage types:
//
//   - "group_stage": a [GroupStage] (round robin groups or lobbies).
//   - "matchpoint": a single lobby of up to MatchSize participants (default
//     all) that replays games until someone has won TargetPoints games
//     (default 1). Each finished game without a champion schedules the next
//     one ("<key>-matchpoint-<round>"); the first is "<key>-matchpoint".
//
// Advance dispatches on Stage.Type; group stages finish when every match is
// resolved, matchpoint stages when the champion is decided.
type MultiStage struct{}

func (MultiStage) Key() string { return KeyMultiStage }

func (MultiStage) Validate(opts Options) error {
	_, err := multiStageSpecs(opts)
	return err
}

func multiStageSpecs(opts Options) ([]StageSpec, error) {
	bad := func(field, reason string) error {
		return &OptionError{Format: KeyMultiStage, Field: field, Reason: reason}
	}
	if len(opts.Stages) == 0 {
		return nil, bad("Stages", "must not be empty")
	}
	specs := make([]StageSpec, len(opts.Stages))
	seen := make(map[string]bool, len(opts.Stages))
	for i, spec := range opts.Stages {
		field := "Stages[" + itoa(i) + "]"
		switch {
		case spec.Key == "":
			return nil, bad(field+".Key", "must not be empty")
		case seen[spec.Key]:
			return nil, bad(field+".Key", "duplicate key "+quote(spec.Key))
		case spec.Source != "" && !seen[spec.Source]:
			return nil, bad(field+".Source", "unknown earlier stage "+quote(spec.Source))
		case spec.Source != "" && spec.Take < 2:
			return nil, bad(field+".Take", "must be at least 2 when Source is set")
		}
		seen[spec.Key] = true
		switch spec.Type {
		case "group_stage":
			o, err := groupOptions(KeyMultiStage, spec.Options, 2, 1)
			if err != nil {
				return nil, err
			}
			spec.Options = o
		case "matchpoint":
			if spec.Options.MatchSize < 0 || spec.Options.MatchSize == 1 {
				return nil, bad(field+".Options.MatchSize", "must be at least 2")
			}
			if spec.Options.TargetPoints < 0 {
				return nil, bad(field+".Options.TargetPoints", "must be at least 1")
			}
			if spec.Options.TargetPoints == 0 {
				spec.Options.TargetPoints = 1
			}
		default:
			return nil, bad(field+".Type", "unsupported stage type "+quote(spec.Type))
		}
		specs[i] = spec
	}
	return specs, nil
}

func (MultiStage) Plan(participants []Participant, opts Options) ([]Stage, error) {
	ids, err := seededIDs(participants)
	if err != nil {
		return nil, err
	}
	specs, err := multiStageSpecs(opts)
	if err != nil {
		return nil, err
	}
	if len(ids) < 2 {
		return nil, &CountError{Format: KeyMultiStage, Rule: AtLeast, Need: 2, Got: len(ids)}
	}
	stages := make([]Stage, 0, len(specs))
	for i, spec := range specs {
		refs, real := ids, true
		if spec.Source != "" {
			refs, real = make([]string, spec.Take), false
			for r := range refs {
				refs[r] = Placeholder(spec.Source, r+1)
			}
		}
		name := spec.Options.Name
		if name == "" {
			name = spec.Name
		}
		if name == "" {
			name = spec.Key
		}
		var st Stage
		switch spec.Type {
		case "group_stage":
			st, err = planGroups(KeyGroupStage, spec.Options, refs, real)
			if err != nil {
				return nil, &StageError{Stage: spec.Key, Err: err}
			}
		case "matchpoint":
			st = planMatchpoint(spec, refs, real)
		}
		st.Key = spec.Key
		st.Name = name
		st.Order = i
		st.Source = spec.Source
		st.SourceCount = spec.Take
		stages = append(stages, st)
	}
	return stages, nil
}

func planMatchpoint(spec StageSpec, refs []string, real bool) Stage {
	size := spec.Options.MatchSize
	if size == 0 {
		size = len(refs)
	}
	if len(refs) > size {
		refs = refs[:size]
	}
	slots := make([]Slot, len(refs))
	for i, ref := range refs {
		slots[i] = makeSlot(ref, real)
	}
	o := spec.Options
	o.MatchSize = size
	return Stage{
		Format:       KeyMultiStage,
		Type:         "matchpoint",
		Options:      o,
		Participants: refs,
		Matches: []Match{{
			ID: spec.Key + "-matchpoint", Round: 1, Number: 1, Slots: slots, Size: size,
		}},
		Scoreboard: map[string]int{},
	}
}

func (MultiStage) Advance(st *Stage, finished *Match) (*Advance, error) {
	if st.Type == "matchpoint" {
		return advanceMatchpoint(st, finished), nil
	}
	return &Advance{Standings: Standings(st), Finished: allResolved(st.Matches)}, nil
}

func advanceMatchpoint(st *Stage, finished *Match) *Advance {
	out := &Advance{}
	if finished == nil {
		return out
	}
	board := make(map[string]int, len(st.Scoreboard)+1)
	for id, v := range st.Scoreboard {
		board[id] = v
	}
	winner := finished.Winner
	if winner != "" {
		board[winner]++
	}
	out.Scoreboard = board
	target := max(st.Options.TargetPoints, 1)
	if winner != "" && board[winner] >= target {
		out.Winner = winner
		out.Finished = allResolved(st.Matches)
		return out
	}
	refs := make([]string, 0, len(finished.Slots))
	for _, s := range finished.Slots {
		if r := s.Ref(); r != "" {
			refs = append(refs, r)
		}
	}
	if len(refs) < 2 {
		refs = st.Participants
	}
	if len(refs) < 2 {
		return out
	}
	next := 1
	for i := range st.Matches {
		if st.Matches[i].Round >= next {
			next = st.Matches[i].Round + 1
		}
	}
	size := st.Options.MatchSize
	if size < 2 {
		size = len(refs)
	}
	key := st.Key
	if key == "" {
		key = "matchpoint"
	}
	slots := make([]Slot, len(refs))
	for i, ref := range refs {
		slots[i] = slotFor(ref)
	}
	out.NewMatches = []Match{{
		ID: key + "-matchpoint-" + itoa(next), Round: next, Number: 1, Slots: slots, Size: size,
	}}
	return out
}

// Qualifiers returns the IDs of the best n participants of a finished stage
// by its standings (see [Standing]); for matchpoint stages the champion
// comes first. n <= 0 returns every ranked participant.
func Qualifiers(st *Stage, n int) []string {
	rows := Standings(st)
	out := make([]string, 0, len(rows))
	if st.Winner != "" {
		out = append(out, st.Winner)
	}
	for _, r := range rows {
		if IsPlaceholder(r.ID) || r.ID == st.Winner {
			continue
		}
		out = append(out, r.ID)
	}
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// FillPlaceholders replaces the placeholders of st that reference source
// with the given qualifiers (qualifiers[0] fills "slot:<source>:1"), in
// participants, groups and match slots. It returns the number of slots
// filled.
func FillPlaceholders(st *Stage, source string, qualifiers []string) int {
	repl := make(map[string]string, len(qualifiers))
	for i, id := range qualifiers {
		repl[Placeholder(source, i+1)] = id
	}
	swap := func(ids []string) {
		for i, ref := range ids {
			if id, ok := repl[ref]; ok {
				ids[i] = id
			}
		}
	}
	swap(st.Participants)
	for g := range st.Groups {
		swap(st.Groups[g].Participants)
	}
	filled := 0
	for mi := range st.Matches {
		for si := range st.Matches[mi].Slots {
			s := &st.Matches[mi].Slots[si]
			if id, ok := repl[s.Placeholder]; ok {
				s.ID, s.Placeholder = id, ""
				filled++
			}
		}
	}
	return filled
}
