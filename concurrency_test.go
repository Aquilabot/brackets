package brackets

import (
	"sync"
	"testing"
)

// Formats and registries are shared across goroutines; every stage is
// owned by one goroutine.
func TestConcurrentUse(t *testing.T) {
	reg := DefaultRegistry()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for _, key := range reg.Keys() {
				f, err := reg.Get(key)
				if err != nil {
					t.Error(err)
					return
				}
				opts := Options{}
				if key == KeyMultiStage {
					opts = majorOptions()
				}
				stages, err := f.Plan(seeded(16), opts)
				if err != nil {
					t.Error(key, err)
					return
				}
				st := &stages[0]
				for !st.Finished {
					i := playable(st)
					if i < 0 {
						break
					}
					r := higherSeedWins(&st.Matches[i])
					if len(st.Matches[i].Slots) > 2 {
						r = Result{Placements: []int{1, 2, 3, 4}[:len(st.Matches[i].Slots)]}
					}
					if _, err := Report(f, st, st.Matches[i].ID, r); err != nil {
						t.Error(key, err)
						return
					}
				}
			}
			reg.Register("custom"+itoa(g), SingleElimination{})
		}(g)
	}
	wg.Wait()
}
