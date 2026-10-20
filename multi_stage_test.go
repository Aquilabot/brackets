package brackets

import (
	"errors"
	"testing"
)

func majorOptions() Options {
	return Options{Stages: []StageSpec{
		{Key: "day1", Name: "Day 1 | Group Stage", Type: "group_stage", Options: Options{GroupSize: 4, MatchSize: 4, Advances: 8}},
		{Key: "phase1", Name: "Day 2 | Phase 1", Type: "group_stage", Source: "day1", Take: 8, Options: Options{GroupSize: 4, MatchSize: 4, Advances: 4}},
		{Key: "finals", Name: "Grand Finals", Type: "matchpoint", Source: "phase1", Take: 4, Options: Options{MatchSize: 4, TargetPoints: 5}},
	}}
}

func TestMultiStageMajorShape(t *testing.T) {
	stages, err := MultiStage{}.Plan(seeded(16), majorOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 3 || stages[0].Name != "Day 1 | Group Stage" || stages[1].Name != "Day 2 | Phase 1" || stages[2].Name != "Grand Finals" {
		t.Fatalf("stages %+v", stages)
	}
	if stages[2].Options.TargetPoints != 5 || stages[2].Matches[0].ID != "finals-matchpoint" {
		t.Fatalf("finals %+v", stages[2])
	}
	for _, s := range stages[1].Matches[0].Slots {
		if s.ID != "" || !IsPlaceholder(s.Placeholder) {
			t.Fatalf("phase1 slots should be placeholders, got %+v", s)
		}
	}
}

func TestMultiStageEndToEnd(t *testing.T) {
	f := MultiStage{}
	stages, err := f.Plan(seeded(16), majorOptions())
	if err != nil {
		t.Fatal(err)
	}
	pick := func(m *Match) Result {
		pl := make([]int, len(m.Slots))
		for i := range pl {
			pl[i] = i + 1
		}
		return Result{Placements: pl}
	}
	for i := range stages {
		st := &stages[i]
		if st.Type == "matchpoint" {
			for !st.Finished {
				id := st.Matches[len(st.Matches)-1].ID
				if _, err := Report(f, st, id, Result{Winner: st.Matches[0].Slots[0].ID}); err != nil {
					t.Fatal(err)
				}
			}
			if st.Scoreboard[st.Winner] != 5 || len(st.Matches) != 5 {
				t.Fatalf("matchpoint: winner %q board %v matches %d", st.Winner, st.Scoreboard, len(st.Matches))
			}
			continue
		}
		playAll(t, f, st, pick)
		if !st.Finished {
			t.Fatalf("stage %s did not finish", st.Key)
		}
		for j := i + 1; j < len(stages); j++ {
			if stages[j].Source == st.Key {
				q := Qualifiers(st, stages[j].SourceCount)
				if len(q) != stages[j].SourceCount {
					t.Fatalf("qualifiers %v", q)
				}
				if filled := FillPlaceholders(&stages[j], st.Key, q); filled == 0 {
					t.Fatalf("nothing filled in %s", stages[j].Key)
				}
			}
		}
	}
}

func TestMultiStageValidation(t *testing.T) {
	bad := []Options{
		{},
		{Stages: []StageSpec{{Key: "", Type: "group_stage"}}},
		{Stages: []StageSpec{{Key: "a", Type: "group_stage"}, {Key: "a", Type: "group_stage"}}},
		{Stages: []StageSpec{{Key: "a", Type: "group_stage", Source: "b", Take: 2}}},
		{Stages: []StageSpec{{Key: "a", Type: "group_stage"}, {Key: "b", Type: "matchpoint", Source: "a", Take: 1}}},
		{Stages: []StageSpec{{Key: "a", Type: "knockout"}}},
		{Stages: []StageSpec{{Key: "a", Type: "matchpoint", Options: Options{MatchSize: 1}}}},
		{Stages: []StageSpec{{Key: "a", Type: "matchpoint", Options: Options{TargetPoints: -1}}}},
		{Stages: []StageSpec{{Key: "a", Type: "group_stage", Options: Options{GroupSize: 1}}}},
	}
	for i, o := range bad {
		if err := (MultiStage{}).Validate(o); !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("case %d: %v", i, err)
		}
	}
	_, err := MultiStage{}.Plan(seeded(5), Options{Stages: []StageSpec{{Key: "g", Type: "group_stage", Options: Options{GroupSize: 4, Strict: true}}}})
	var se *StageError
	var ce *CountError
	if !errors.As(err, &se) || se.Stage != "g" || !errors.As(err, &ce) || ce.Rule != MultipleOf {
		t.Fatalf("stage error: %v", err)
	}
}

func TestRegistry(t *testing.T) {
	r := DefaultRegistry()
	keys := r.Keys()
	if len(keys) != 7 {
		t.Fatalf("keys %v", keys)
	}
	for _, k := range keys {
		f, err := r.Get(k)
		if err != nil || f.Key() != k {
			t.Fatalf("Get(%s) = %v, %v", k, f, err)
		}
	}
	if _, err := r.Get("nope"); !errors.Is(err, ErrUnknownFormat) {
		t.Fatalf("unknown: %v", err)
	}
	r.Register("ko", SingleElimination{})
	if f, err := r.Get("ko"); err != nil || f.Key() != KeySingleElimination {
		t.Fatal("custom key")
	}
}
