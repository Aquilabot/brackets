package brackets

import "strings"

// Participant is an entrant of a tournament: a player, a team, a roster.
//
// Seed ranks the participant before play starts. Seeds greater than zero are
// ordered ascending (1 is the strongest); a Seed of zero or less means
// "unseeded" and those participants are placed after every seeded one, in
// the order they were given. Ties keep input order.
type Participant struct {
	ID   string
	Seed int
}

// Status is the lifecycle state of a match.
type Status uint8

const (
	// Pending matches have not started. Unresolved slots may still be empty.
	Pending Status = iota
	// Live matches are being played.
	Live
	// Finished matches have a result (a winner, or a draw when Winner is "").
	Finished
	// Cancelled matches will never be played (for example a phantom bracket
	// match whose two feeders were both byes).
	Cancelled
)

// Resolved reports whether the match no longer blocks progression.
func (s Status) Resolved() bool { return s != Pending && s != Live }

func (s Status) String() string {
	switch s {
	case Pending:
		return "pending"
	case Live:
		return "live"
	case Finished:
		return "finished"
	case Cancelled:
		return "cancelled"
	}
	return "unknown"
}

// Bracket identifies the side of a double-elimination bracket a match
// belongs to. Formats without sides leave it as NoBracket.
type Bracket uint8

const (
	NoBracket Bracket = iota
	UpperBracket
	LowerBracket
	GrandFinal
)

func (b Bracket) String() string {
	switch b {
	case UpperBracket:
		return "upper"
	case LowerBracket:
		return "lower"
	case GrandFinal:
		return "grand_final"
	}
	return ""
}

// PlaceholderPrefix marks slot references that will be filled by the
// qualifiers of an earlier stage (see [Placeholder]). Participant IDs must
// not start with it.
const PlaceholderPrefix = "slot:"

// Placeholder returns the reference used for the rank-th qualifier
// (1-based) of the stage with the given key, for example "slot:groups:3".
func Placeholder(stage string, rank int) string {
	return PlaceholderPrefix + stage + ":" + itoa(rank)
}

// IsPlaceholder reports whether ref is a placeholder produced by
// [Placeholder] rather than a participant ID.
func IsPlaceholder(ref string) bool { return strings.HasPrefix(ref, PlaceholderPrefix) }

// Slot is one seat in a match. Head-to-head matches have exactly two slots
// (index 0 is "home", index 1 is "away"); free-for-all lobbies have one slot
// per participant.
type Slot struct {
	// ID is the participant occupying the slot, or "" while unresolved.
	ID string
	// Placeholder is set instead of ID when the seat will be filled by a
	// qualifier from an earlier stage.
	Placeholder string

	// Score is the raw score of the slot: maps/goals/rounds in head-to-head
	// matches, an arbitrary score in lobbies.
	Score int
	// Points are ranking points earned in a lobby. When HasPoints is set,
	// standings use Points instead of Score as "points for".
	Points    int
	HasPoints bool
	// Placement is the 1-based finishing position in a lobby (0 = not
	// reported). Placement 1 counts as a win, any other placement as a loss.
	Placement int
}

// Ref returns the participant ID, or the placeholder when unresolved.
func (s Slot) Ref() string {
	if s.ID != "" {
		return s.ID
	}
	return s.Placeholder
}

// Link points at a slot of another match: where the winner (or the loser)
// of a match goes next.
type Link struct {
	Match string // target match ID
	Slot  int    // 0 = home, 1 = away
	// Index is a hint: the position of the target in Stage.Matches when the
	// link was created. It is verified against Match before use, so a stale
	// or zero hint only costs a lookup by ID.
	Index int
}

// Match is a single game or series between the participants in Slots.
//
// Match IDs are deterministic and unique within a stage: the same input
// always yields the same IDs, so callers can use them as stable keys.
type Match struct {
	ID     string
	Round  int // schedule column; 1-based
	Number int // 1-based position within the round
	Slots  []Slot
	// Winner is the winning participant ID. An empty winner on a finished
	// head-to-head match is a draw.
	Winner string
	Status Status

	// Bracket structure (elimination formats).
	Bracket     Bracket
	Elimination bool // losing this match eliminates the loser
	// Bye is set on matches resolved at planning time because one side was
	// empty; Phantom on matches cancelled because both sides were empty.
	Bye     bool
	Phantom bool
	Partial bool // lobby with fewer participants than the configured size

	BracketRound int // round inside Bracket (upper/lower are counted separately)
	BestOf       int // series length hint (1, 3, 5); 0 = unspecified
	WinnerTo     *Link
	LoserTo      *Link
	// From lists the IDs of the matches whose results fill this match.
	From []string

	// Group structure (group stages, round robin, lobby leagues).
	Group      string // group name ("A", "B", ...); "" when not grouped
	GroupIndex int
	Leg        int // 1-based leg for multi-leg round robins
	LobbyRound int // 1-based lobby round for free-for-all lobbies
	Size       int // configured participants per match (lobby size)
}

// Home returns the participant in slot 0, or "".
func (m *Match) Home() string {
	if len(m.Slots) > 0 {
		return m.Slots[0].ID
	}
	return ""
}

// Away returns the participant in slot 1, or "".
func (m *Match) Away() string {
	if len(m.Slots) > 1 {
		return m.Slots[1].ID
	}
	return ""
}

// Loser returns the first slot participant that is not the winner, or ""
// when the match has no winner.
func (m *Match) Loser() string {
	if i := m.LoserSlot(); i >= 0 {
		return m.Slots[i].ID
	}
	return ""
}

// LoserSlot returns the index of the slot Loser reads, or -1.
func (m *Match) LoserSlot() int {
	if m.Winner == "" {
		return -1
	}
	for i := range m.Slots {
		if id := m.Slots[i].ID; id != "" && id != m.Winner {
			return i
		}
	}
	return -1
}

// Has reports whether id occupies one of the match slots.
func (m *Match) Has(id string) bool {
	for _, s := range m.Slots {
		if s.ID == id {
			return true
		}
	}
	return false
}

// Group is a pool of participants inside a group stage.
type Group struct {
	Name         string
	Participants []string
}

// Bye records a participant that sat out a round and was credited a win.
type Bye struct {
	Round       int
	Participant string
}

// Stage is one phase of a tournament: its configuration, its participants
// and its matches. A Stage is plain data; persist it however you like and
// pass it back to [Format.Advance] when a match finishes.
type Stage struct {
	Key    string // stage key inside a multi-stage plan; "" otherwise
	Name   string
	Format string // key of the format that planned the stage
	Type   string // structural kind: "bracket", "double_elimination", "swiss", "round_robin", "group", "ffa_league", "matchpoint"
	Order  int    // 0-based position inside the plan

	Options      Options
	Participants []string // participant IDs (or placeholders) in seed order
	Groups       []Group
	Matches      []Match

	Rounds       int // total rounds when known up front (0 = derive from matches)
	CurrentRound int
	Size         int   // bracket size (elimination formats)
	SeedOrder    []int // seed expected in each first-round slot (elimination formats)
	UpperSize    int   // double elimination: participants starting in the upper bracket
	UpperRounds  int
	LowerRounds  int

	// Source and SourceCount are set when the stage is fed by the
	// SourceCount best participants of an earlier stage.
	Source      string
	SourceCount int

	Byes       []Bye
	Standings  []Standing
	Scoreboard map[string]int // matchpoint: games won per participant
	Winner     string
	Finished   bool
}

// Match returns a pointer to the match with the given ID, or nil.
func (s *Stage) Match(id string) *Match {
	for i := range s.Matches {
		if s.Matches[i].ID == id {
			return &s.Matches[i]
		}
	}
	return nil
}
