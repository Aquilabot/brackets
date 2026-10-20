package brackets

import (
	"fmt"
	"slices"
	"sync"
)

// Built-in format keys.
const (
	KeySingleElimination = "single_elimination"
	KeyDoubleElimination = "double_elimination"
	KeySwiss             = "swiss"
	KeyRoundRobin        = "round_robin"
	KeyGroupStage        = "group_stage"
	KeyFFALeague         = "ffa_league"
	KeyMultiStage        = "multi_stage"
)

// Format is the strategy every competitive structure implements.
//
// Formats are stateless and safe for concurrent use: all state lives in the
// [Stage] values they return, which the caller stores and hands back.
type Format interface {
	// Key is the stable identifier the format is registered under.
	Key() string

	// Validate checks options without planning anything.
	Validate(opts Options) error

	// Plan seeds the participants and returns the initial stages with their
	// matches. Matches whose participants are not known yet (later bracket
	// rounds, later Swiss rounds, stages fed by earlier stages) have empty
	// or placeholder slots. Byes and phantom matches are resolved up front.
	// The output depends only on the inputs: same participants and options,
	// same stages, byte for byte.
	Plan(participants []Participant, opts Options) ([]Stage, error)

	// Advance is called after a match of stage finished (its Status, Winner
	// and slot results already recorded in stage.Matches, and passed again
	// as finished). It does not modify stage; it returns the effects: slots
	// to fill, new matches to schedule, updated standings and whether the
	// stage is complete. Use [Stage.Apply] to apply them, or [Report] to do
	// all of it in one call.
	Advance(stage *Stage, finished *Match) (*Advance, error)
}

// Advance is the outcome of a finished match.
type Advance struct {
	// Updates fill slots of existing matches (bracket progression).
	Updates []SlotUpdate
	// NewMatches are matches to append to the stage (next Swiss round, next
	// matchpoint game).
	NewMatches []Match
	// Standings are the recomputed standings, for formats that keep them.
	Standings []Standing
	// NewByes are byes granted while scheduling NewMatches.
	NewByes []Bye
	// CurrentRound is the round the stage moved to (0 = unchanged).
	CurrentRound int
	// Scoreboard is the updated matchpoint scoreboard (nil = unchanged).
	Scoreboard map[string]int
	// Winner is set when the stage produced a winner.
	Winner string
	// Finished reports that every match of the stage is resolved (or, for
	// formats that end on a specific match, that the deciding match ended).
	Finished bool
}

// SlotUpdate places a participant into a slot of an existing match.
type SlotUpdate struct {
	Index       int    // index of the match in Stage.Matches
	Match       string // ID of the match
	Slot        int    // 0 = home, 1 = away
	Participant string
}

// Registry maps format keys to formats. The zero value is not usable; use
// [NewRegistry] or [DefaultRegistry].
type Registry struct {
	mu      sync.RWMutex
	formats map[string]Format
}

// NewRegistry returns a registry with the given formats. Later formats
// replace earlier ones with the same key.
func NewRegistry(formats ...Format) *Registry {
	r := &Registry{formats: make(map[string]Format, len(formats))}
	for _, f := range formats {
		r.formats[f.Key()] = f
	}
	return r
}

// DefaultRegistry returns a new registry with every built-in format.
func DefaultRegistry() *Registry {
	return NewRegistry(
		SingleElimination{},
		DoubleElimination{},
		Swiss{},
		RoundRobin{},
		GroupStage{},
		FFALeague{},
		MultiStage{},
	)
}

// Register adds f under key (or under f.Key() when key is empty),
// replacing any previous format with that key.
func (r *Registry) Register(key string, f Format) {
	if key == "" {
		key = f.Key()
	}
	r.mu.Lock()
	r.formats[key] = f
	r.mu.Unlock()
}

// Get returns the format registered under key.
func (r *Registry) Get(key string) (Format, error) {
	r.mu.RLock()
	f, ok := r.formats[key]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownFormat, key)
	}
	return f, nil
}

// Keys returns the registered keys, sorted.
func (r *Registry) Keys() []string {
	r.mu.RLock()
	keys := make([]string, 0, len(r.formats))
	for k := range r.formats {
		keys = append(keys, k)
	}
	r.mu.RUnlock()
	slices.Sort(keys)
	return keys
}
