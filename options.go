package brackets

import "math/rand/v2"

// Seeding controls how participants are distributed into groups.
type Seeding uint8

const (
	// Snake distributes seeds A,B,C,C,B,A,A,B,C,... so every group gets a
	// comparable mix of strong and weak seeds. Default.
	Snake Seeding = iota
	// Sequential fills group A first, then B, and so on.
	Sequential
)

func (s Seeding) String() string {
	if s == Sequential {
		return "sequential"
	}
	return "snake"
}

// PartialPolicy controls whether lobbies with fewer participants than
// Options.MatchSize are scheduled.
type PartialPolicy uint8

const (
	// PartialAuto allows partial lobbies unless Options.Strict is set.
	PartialAuto PartialPolicy = iota
	PartialAllow
	PartialDeny
)

// Pairing selects the Swiss pairing algorithm.
type Pairing uint8

const (
	// AvoidRematches (default) pairs participants in standings order, each
	// with the highest-ranked opponent they have not met, and backtracks
	// when that greedy choice would force a rematch later in the round.
	// A rematch is only produced when no rematch-free pairing exists for the
	// round (or the search budget is exhausted), in which case the result is
	// the Greedy pairing.
	AvoidRematches Pairing = iota
	// Greedy pairs the best remaining participant with the best remaining
	// opponent they have not met yet, or with the next participant in the
	// standings if they have met everyone left. Never backtracks, so it can
	// produce avoidable rematches near the bottom of the standings.
	Greedy
)

// Options configures a format. Every field is optional; zero values select
// the documented defaults and fields a format does not use are ignored.
type Options struct {
	// Name labels the planned stage. Defaults to a per-format name.
	Name string

	// Rounds is the number of Swiss rounds. Default: ceil(log2(n)), 1 for
	// two participants.
	Rounds int
	// Pairing selects the Swiss pairing algorithm. Default AvoidRematches.
	Pairing Pairing
	// Shuffle randomizes the order of Swiss participants whose records are
	// identical before each round is paired (including round 1, which is
	// otherwise paired 1v2, 3v4, ... by seed). Randomness comes from Rand,
	// or from a PCG generator seeded with (Seed, round) when Rand is nil,
	// so a persisted Seed reproduces every round exactly.
	Shuffle bool
	Seed    uint64
	Rand    rand.Source

	// Legs is how many times each pair meets in round robin and group
	// stages (home and away swap on even legs). Default 1.
	Legs int
	// GroupSize is the number of participants per group. Default 4.
	GroupSize int
	// MatchSize is the number of participants per match: 2 for head-to-head
	// round robin groups, more for free-for-all lobbies. Default 2 for
	// group stages, 4 for FFA leagues, every participant for matchpoint.
	MatchSize int
	// LobbyRounds is the number of lobby rounds when MatchSize > 2.
	// Default 1 for group stages, 4 for FFA leagues.
	LobbyRounds int
	// Seeding distributes participants into groups. Default Snake.
	Seeding Seeding
	// Strict requires the participant count to be a multiple of GroupSize.
	Strict bool
	// Partial controls partial lobbies. Default PartialAuto.
	Partial PartialPolicy
	// Advances and AdvancesPerGroup are informational qualification counts
	// carried on the stage for callers that chain stages.
	Advances         int
	AdvancesPerGroup int

	// TargetPoints is the number of game wins a participant needs to take a
	// matchpoint stage. Default 1.
	TargetPoints int

	// Stages describes the stages of a multi-stage format, in order.
	Stages []StageSpec
}

// StageSpec describes one stage of a [MultiStage] format.
type StageSpec struct {
	// Key identifies the stage; later stages reference it in Source.
	Key string
	// Name labels the stage. Defaults to Options.Name, then Key.
	Name string
	// Type is "group_stage" or "matchpoint".
	Type string
	// Source is the key of an earlier stage whose top Take participants
	// fill this stage through placeholders. Empty means "every participant".
	Source string
	Take   int
	// Options configures the stage (group sizes, lobby rounds, target
	// points, ...).
	Options Options
}

// DefaultSwissRounds returns the default number of Swiss rounds for n
// participants: ceil(log2(n)), and 1 for n <= 2.
func DefaultSwissRounds(n int) int {
	if n <= 2 {
		return 1
	}
	r := 0
	for p := 1; p < n; p <<= 1 {
		r++
	}
	return r
}
