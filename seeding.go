package brackets

import (
	"slices"
	"strconv"
)

// SortParticipants returns a copy of ps in seed order: seeds > 0 ascending,
// then unseeded participants (Seed <= 0). The sort is stable, so equal seeds
// and unseeded participants keep their input order.
func SortParticipants(ps []Participant) []Participant {
	out := slices.Clone(ps)
	slices.SortStableFunc(out, compareSeed)
	return out
}

func compareSeed(a, b Participant) int {
	as, bs := a.Seed > 0, b.Seed > 0
	switch {
	case as && bs:
		return a.Seed - b.Seed
	case as:
		return -1
	case bs:
		return 1
	}
	return 0
}

// seededIDs validates participants and returns their IDs in seed order.
func seededIDs(ps []Participant) ([]string, error) {
	sorted := ps
	if !slices.IsSortedFunc(ps, compareSeed) {
		sorted = SortParticipants(ps)
	}
	ids := make([]string, len(sorted))
	for i, p := range sorted {
		switch {
		case p.ID == "":
			return nil, &ParticipantError{ID: p.ID, Reason: "empty id"}
		case IsPlaceholder(p.ID):
			return nil, &ParticipantError{ID: p.ID, Reason: "reserved placeholder prefix"}
		}
		ids[i] = p.ID
	}
	// Duplicate check on a sorted copy: cheaper than a map for the sizes
	// tournaments have.
	check := slices.Clone(ids)
	slices.Sort(check)
	for i := 1; i < len(check); i++ {
		if check[i] == check[i-1] {
			return nil, &ParticipantError{ID: check[i], Reason: "duplicate id"}
		}
	}
	return ids, nil
}

// NextPowerOfTwo returns the smallest power of two >= n (1 for n <= 1).
func NextPowerOfTwo(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

func isPowerOfTwo(n int) bool { return n > 0 && n&(n-1) == 0 }

func log2(n int) int {
	r := 0
	for n > 1 {
		n >>= 1
		r++
	}
	return r
}

// SeedOrder returns the seed expected in each first-round slot of a
// standard elimination bracket of the given power-of-two size. Adjacent
// slots meet in round 1, and the top seeds cannot meet before the rounds
// their seeding earns them: for 8 slots it returns [1 8 4 5 2 7 3 6]
// (quarterfinals 1v8, 4v5, 2v7, 3v6).
func SeedOrder(size int) []int {
	if size <= 1 {
		return []int{1}
	}
	order := make([]int, size)
	order[0], order[1] = 1, 2
	for n := 2; n < size; n *= 2 {
		// Expand in place from the back so every seed s at position i becomes
		// the pair (s, 2n+1-s) at positions 2i, 2i+1.
		for i := n - 1; i >= 0; i-- {
			s := order[i]
			order[2*i] = s
			order[2*i+1] = 2*n + 1 - s
		}
	}
	return order
}

// GroupName returns the conventional name of the i-th group (0-based):
// "A".."Z", then "G27", "G28", ...
func GroupName(i int) string {
	if i >= 0 && i < 26 {
		return string(rune('A' + i))
	}
	return "G" + strconv.Itoa(i+1)
}

func itoa(n int) string { return strconv.Itoa(n) }

// idBuilder accumulates many short IDs in one buffer and slices them out of
// a single string, so planning N matches costs two allocations for IDs
// instead of N.
type idBuilder struct {
	buf  []byte
	ends []int
}

func newIDBuilder(n, avg int) *idBuilder {
	return &idBuilder{buf: make([]byte, 0, n*avg), ends: make([]int, 0, n)}
}

func (b *idBuilder) s(v string) *idBuilder { b.buf = append(b.buf, v...); return b }
func (b *idBuilder) n(v int) *idBuilder    { b.buf = strconv.AppendInt(b.buf, int64(v), 10); return b }
func (b *idBuilder) end()                  { b.ends = append(b.ends, len(b.buf)) }

func (b *idBuilder) build() []string {
	all := string(b.buf)
	out := make([]string, len(b.ends))
	start := 0
	for i, e := range b.ends {
		out[i] = all[start:e]
		start = e
	}
	return out
}
