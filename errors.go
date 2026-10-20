package brackets

import "errors"

// Sentinel errors. Every error returned by this package wraps one of them,
// so callers can branch with errors.Is and inspect details with errors.As.
var (
	// ErrUnknownFormat is returned by [Registry.Get] for unregistered keys.
	ErrUnknownFormat = errors.New("brackets: unknown format")
	// ErrInvalidOptions wraps every [*OptionError].
	ErrInvalidOptions = errors.New("brackets: invalid options")
	// ErrParticipantCount wraps every [*CountError].
	ErrParticipantCount = errors.New("brackets: unsupported number of participants")
	// ErrInvalidParticipant wraps every [*ParticipantError].
	ErrInvalidParticipant = errors.New("brackets: invalid participant")
	// ErrMatchNotFound is returned when a match (or the target of a
	// winner/loser link) does not exist in the stage.
	ErrMatchNotFound = errors.New("brackets: match not found")
	// ErrInvalidStage is returned when a stage lacks the data a format needs
	// to advance it.
	ErrInvalidStage = errors.New("brackets: invalid stage")
	// ErrInvalidResult is returned by [Report] for results that cannot be
	// applied to the match.
	ErrInvalidResult = errors.New("brackets: invalid result")
)

// OptionError reports an invalid [Options] field.
type OptionError struct {
	Format string // format key
	Field  string // Options field (or "Stages[i].Field")
	Reason string
}

func (e *OptionError) Error() string {
	return "brackets: " + e.Format + ": invalid " + e.Field + ": " + e.Reason
}

func (e *OptionError) Unwrap() error { return ErrInvalidOptions }

// CountRule is the participant-count constraint a [CountError] violated.
type CountRule uint8

const (
	// AtLeast: fewer than Need participants.
	AtLeast CountRule = iota
	// PowerOfTwo: the format needs a power-of-two participant count.
	PowerOfTwo
	// MultipleOf: the count must be a multiple of Need (strict groups).
	MultipleOf
	// NoPlayableMatches: the configuration produced no playable match.
	NoPlayableMatches
)

// CountError reports a participant count the format cannot schedule.
type CountError struct {
	Format string
	Rule   CountRule
	Need   int
	Got    int
}

func (e *CountError) Error() string {
	switch e.Rule {
	case PowerOfTwo:
		return "brackets: " + e.Format + " needs a power-of-two number of participants, got " + itoa(e.Got)
	case MultipleOf:
		return "brackets: " + e.Format + " needs a multiple of " + itoa(e.Need) + " participants, got " + itoa(e.Got)
	case NoPlayableMatches:
		return "brackets: " + e.Format + " produced no playable matches for " + itoa(e.Got) + " participants"
	}
	return "brackets: " + e.Format + " needs at least " + itoa(e.Need) + " participants, got " + itoa(e.Got)
}

func (e *CountError) Unwrap() error { return ErrParticipantCount }

// ParticipantError reports an invalid participant ID.
type ParticipantError struct {
	ID     string
	Reason string // "empty id", "duplicate id", "reserved placeholder prefix"
}

func (e *ParticipantError) Error() string {
	return "brackets: participant " + quote(e.ID) + ": " + e.Reason
}

func (e *ParticipantError) Unwrap() error { return ErrInvalidParticipant }

// StageError wraps an error raised while planning one stage of a
// multi-stage format.
type StageError struct {
	Stage string // stage key
	Err   error
}

func (e *StageError) Error() string {
	return "brackets: stage " + quote(e.Stage) + ": " + e.Err.Error()
}
func (e *StageError) Unwrap() error { return e.Err }

// LinkError reports that the match a result should flow into is missing.
type LinkError struct {
	From   string // finished match ID
	Target string // target match ID, when known
	Round  int    // target round, for formats that address matches by round/number
	Number int
}

func (e *LinkError) Error() string {
	if e.Target != "" {
		return "brackets: no downstream match " + quote(e.Target) + " for " + quote(e.From)
	}
	return "brackets: no downstream match at round " + itoa(e.Round) + " match " + itoa(e.Number) + " for " + quote(e.From)
}

func (e *LinkError) Unwrap() error { return ErrMatchNotFound }

func quote(s string) string { return "\"" + s + "\"" }
