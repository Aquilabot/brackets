package brackets

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorMessages(t *testing.T) {
	cases := []struct {
		err      error
		sentinel error
		contains string
	}{
		{&OptionError{Format: "swiss", Field: "Rounds", Reason: "must not be negative"}, ErrInvalidOptions, "swiss: invalid Rounds"},
		{&CountError{Format: "x", Rule: AtLeast, Need: 2, Got: 1}, ErrParticipantCount, "at least 2"},
		{&CountError{Format: "x", Rule: PowerOfTwo, Got: 6}, ErrParticipantCount, "power-of-two"},
		{&CountError{Format: "x", Rule: MultipleOf, Need: 4, Got: 5}, ErrParticipantCount, "multiple of 4"},
		{&CountError{Format: "x", Rule: NoPlayableMatches, Got: 3}, ErrParticipantCount, "no playable"},
		{&ParticipantError{ID: "a", Reason: "duplicate id"}, ErrInvalidParticipant, `"a": duplicate`},
		{&StageError{Stage: "g", Err: &CountError{Format: "x", Need: 2}}, ErrParticipantCount, `stage "g"`},
		{&LinkError{From: "a", Target: "b"}, ErrMatchNotFound, `"b"`},
		{&LinkError{From: "a", Round: 2, Number: 1}, ErrMatchNotFound, "round 2 match 1"},
		{&resultError{match: "m", reason: "bad"}, ErrInvalidResult, `"m": bad`},
		{errNoRounds, ErrInvalidStage, "round count"},
	}
	for _, c := range cases {
		if !errors.Is(c.err, c.sentinel) || !strings.Contains(c.err.Error(), c.contains) || !strings.HasPrefix(c.err.Error(), "brackets: ") {
			t.Errorf("%T: %q (is %v)", c.err, c.err.Error(), errors.Is(c.err, c.sentinel))
		}
	}
}

func TestStringers(t *testing.T) {
	for s, want := range map[Status]string{Pending: "pending", Live: "live", Finished: "finished", Cancelled: "cancelled", 9: "unknown"} {
		if s.String() != want {
			t.Errorf("%d: %s", s, s)
		}
	}
	if NoBracket.String() != "" || Snake.String() != "snake" || Sequential.String() != "sequential" {
		t.Error("stringers")
	}
	if GroupName(0) != "A" || GroupName(25) != "Z" || GroupName(26) != "G27" || GroupName(-1) != "G0" {
		t.Error("GroupName")
	}
	var m Match
	if m.Home() != "" || m.Away() != "" || m.Loser() != "" || m.Has("x") {
		t.Error("empty match accessors")
	}
	if (&Stage{}).Match("x") != nil {
		t.Error("Stage.Match")
	}
}

func TestValidateDefaults(t *testing.T) {
	for _, f := range []Format{SingleElimination{}, DoubleElimination{}, Swiss{}, RoundRobin{}, GroupStage{}, FFALeague{}} {
		if err := f.Validate(Options{}); err != nil {
			t.Errorf("%s: zero options should be valid: %v", f.Key(), err)
		}
	}
	if err := (RoundRobin{}).Validate(Options{Legs: -1}); !errors.Is(err, ErrInvalidOptions) {
		t.Error("round robin legs")
	}
	if err := (FFALeague{}).Validate(Options{GroupSize: 1}); !errors.Is(err, ErrInvalidOptions) {
		t.Error("ffa group size")
	}
	if err := (Swiss{}).Validate(Options{Pairing: 7}); !errors.Is(err, ErrInvalidOptions) {
		t.Error("swiss pairing")
	}
	for _, f := range []Format{RoundRobin{}, GroupStage{}, FFALeague{}, MultiStage{}, Swiss{}, DoubleElimination{}} {
		if _, err := f.Plan(seeded(1), Options{Stages: majorOptions().Stages}); err == nil {
			t.Errorf("%s: one participant should fail", f.Key())
		}
		if _, err := f.Plan([]Participant{{ID: ""}, {ID: "b"}}, Options{Stages: majorOptions().Stages}); !errors.Is(err, ErrInvalidParticipant) {
			t.Errorf("%s: empty id should fail: %v", f.Key(), err)
		}
	}
}
