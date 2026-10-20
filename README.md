# brackets

Tournament formats for Go: seeding, brackets, byes, Swiss pairings, round
robins, group stages, free-for-all lobby leagues and multi-stage flows.

You give `brackets` participants and options; it returns a deterministic
schedule of matches. You report results; it tells you which slots to fill,
which matches to add, the standings and when a stage is over. Stages are
plain values you can store anywhere, formats are stateless and safe for
concurrent use, and the module uses only the Go standard library.

## Formats

| Key | Type | What it does |
| --- | --- | --- |
| `single_elimination` | `SingleElimination` | Knockout bracket sized to the next power of two. Standard seed placement (1v8, 4v5, 2v7, 3v6, ...) so top seeds meet as late as possible; empty slots become byes for the top seeds and are resolved at planning time. |
| `double_elimination` | `DoubleElimination` | Upper and lower bracket for a power-of-two field (at least 4). The top half of the seeds starts in the upper bracket, the bottom half in the lower bracket; upper losers drop into the lower bracket; one loss in the lower bracket eliminates. Lower round 1 is best-of-1, the grand final best-of-5 with no advantage. |
| `swiss` | `Swiss` | Fixed number of rounds (default `ceil(log2 n)`), each paired from the current standings once the previous round is complete. Odd fields give a bye (counted as a win) to the lowest-ranked participant without one. Rematch-avoiding pairing by default. |
| `round_robin` | `RoundRobin` | Everyone meets everyone, `Legs` times (home/away swapped on even legs), scheduled with the circle method so nobody plays twice in a round. Keeps standings and a winner. |
| `group_stage` | `GroupStage` | Splits the field into groups (snake or sequential seeding). Head-to-head groups play a round robin; with `MatchSize > 2` groups play rotating free-for-all lobbies. Optional strict group sizes and partial lobbies. |
| `ffa_league` | `FFALeague` | Free-for-all lobby league (battle royale / arena style): groups of 4 play 4 lobby rounds of 4-participant lobbies by default. Results are placements and points per slot. |
| `multi_stage` | `MultiStage` | Chains stages: group stages and "matchpoint" finals (replayed until someone wins `TargetPoints` games). Later stages are seeded with placeholders that you fill with the qualifiers of earlier stages. |

## Install

```sh
go get github.com/Aquilabot/brackets
```

Requires Go 1.23 or newer.

## Example

```go
package main

import (
	"fmt"

	"github.com/Aquilabot/brackets"
)

func main() {
	players := []brackets.Participant{
		{ID: "ana", Seed: 1}, {ID: "ben", Seed: 2}, {ID: "cleo", Seed: 3},
		{ID: "dan", Seed: 4}, {ID: "eve", Seed: 5}, {ID: "fay", Seed: 6},
	}

	f, err := brackets.DefaultRegistry().Get("single_elimination")
	if err != nil {
		panic(err)
	}
	stages, err := f.Plan(players, brackets.Options{})
	if err != nil {
		panic(err)
	}
	st := &stages[0] // plain data: store it however you like

	for _, m := range st.Matches {
		switch {
		case m.Bye:
			fmt.Printf("%s: bye for %s\n", m.ID, m.Winner)
		case m.Round == 1:
			fmt.Printf("%s: %s vs %s\n", m.ID, m.Home(), m.Away())
		}
	}

	// Play the bracket: report every match whose two slots are filled.
	for !st.Finished {
		for i := range st.Matches {
			m := &st.Matches[i]
			if m.Status != brackets.Pending || m.Home() == "" || m.Away() == "" {
				continue
			}
			result := brackets.Result{Winner: m.Home(), Scores: []int{2, 0}}
			if _, err := brackets.Report(f, st, m.ID, result); err != nil {
				panic(err)
			}
		}
	}
	fmt.Println("champion:", st.Winner)
}
```

Output:

```
r1-m1: bye for ana
r1-m2: dan vs eve
r1-m3: bye for ben
r1-m4: cleo vs fay
champion: ana
```

`Report` validates the result (match exists and is open, every slot is
filled, the winner plays in it, elimination matches have a winner), records
it, calls `Format.Advance` and applies the outcome to the stage. If you keep
matches in your own storage, call `Advance` yourself: it does not modify the
stage, it returns an `Advance` value with the slots to fill (`Updates`), the
matches to create (`NewMatches`), the standings, byes, winner and whether the
stage finished.

More examples (Swiss, registry lookups) are in
[example_test.go](example_test.go); multi-stage chaining with `Qualifiers`
and `FillPlaceholders` is exercised in
[multi_stage_test.go](multi_stage_test.go).

## Concepts

- **Participant**: `ID` plus optional `Seed`. Seeds `> 0` sort ascending;
  unseeded participants (`Seed <= 0`) go last in input order. IDs must be
  unique, non-empty and must not start with `slot:` (reserved for
  placeholders).
- **Stage**: one phase: its effective `Options`, participants, groups,
  matches, byes, standings, scoreboard and winner.
- **Match**: `ID`, `Round`, `Number`, `Slots` (two for head-to-head, home
  first; N for lobbies), `Status`, `Winner`, plus structure: bracket side,
  best-of hint, `WinnerTo`/`LoserTo` links, feeder `From` IDs, group, leg,
  lobby round, bye/phantom/partial flags.
- **Format**: `Plan(participants, options) ([]Stage, error)` and
  `Advance(stage, finishedMatch) (*Advance, error)`. Formats are stateless
  and safe for concurrent use.
- **Registry**: formats by key. `DefaultRegistry()` has all seven; register
  your own `Format` under any key.
- **Tally**: for matches kept in your own storage. Feed each stored match
  through one reused `Match` value with `Tally.Add`, then read
  `Tally.Standings()` or call `Swiss{}.AdvanceTally(stage, tally)`; the
  stage only needs its configuration, not its matches. Bracket progression
  has the same kind of entry points: `SingleElimination{}.Parent(round,
  number)` gives the next match and slot, `DoubleElimination{}.Moves(match)`
  gives where the winner and loser go.

## Determinism

- `Plan` is a pure function of its inputs: the same participants and options
  produce the same stages, match IDs, slots and links, every time and on
  every platform. No map iteration order or clock is involved.
- Match IDs are stable and human-readable (`r2-m1`, `upper-r1-m3`,
  `lower-r4-m1`, `grand-final`, `swiss-r3-m7`, `gA-r2-m1`, `gB-r1-lobby2`,
  `finals-matchpoint-4`), unique within a stage.
- `Advance` depends only on the stage and the finished match. It recomputes
  standings from the recorded results on every call, so a corrected result
  is reflected the next time it runs.
- Swiss is deterministic by default (round 1 pairs adjacent seeds). With
  `Options.Shuffle`, participants with identical records are shuffled before
  pairing using `Options.Rand` if given, otherwise a PCG generator seeded
  with `(Options.Seed, round)`. Persist `Seed` and every round can be
  reproduced exactly.

## Byes

- Elimination: the bracket is padded to a power of two and the empty slots
  face the top seeds, so with `n` participants and bracket size `S` seeds
  `1..S-n` advance without playing. Bye matches are marked `Bye`, already
  `Finished`, and their winners are placed in round 2.
- Swiss: with an odd field, the lowest-ranked participant who has not had a
  bye sits out and is credited a win; a second bye is only given once
  everyone has had one.
- Round robin with an odd field: one participant rests each round.
- Lobbies: groups that do not divide evenly produce partial lobbies (flagged
  `Partial`) unless `Options.Strict` or `PartialDeny` is set.

## Standings and tie-breakers

`Standings(stage)` (and the `Standings` returned by `Advance`) ranks
participants by:

1. Wins, descending (a bye counts as a win).
2. Losses, ascending.
3. Score difference (points for minus points against), descending.
4. Points for, descending.
5. Seed order, ascending (position in the stage's participant list; anyone
   first seen in results comes after, in order of appearance).

Only finished matches count. Head-to-head: an empty winner is a draw, each
side scores its own `Score` for and the opponent's against. Lobbies: points
for are the slot's `Points` (or `Score` if no points were reported), placement
1 is a win and any other placement a loss; without placements the match
winner decides. `GroupStandings` gives one table per group.

## Swiss pairing

- `AvoidRematches` (default): participants are taken in standings order and
  each is paired with the highest-ranked opponent they have not met. If that
  greedy choice would force a rematch later in the round, the search
  backtracks (depth-first, bounded to about a million candidate checks per
  round). A rematch only happens when no rematch-free pairing of the round
  exists, or the budget runs out; then the `Greedy` pairing is used.
- `Greedy`: the same order without backtracking. It never produces a
  rematch while an unmet opponent is still available for the participant
  being paired, but it can corner the bottom of the table into a rematch
  that a different pairing would have avoided.

The bye is chosen before pairing and is not reconsidered.

## Errors

Every error wraps a sentinel, so `errors.Is` works, and carries details for
`errors.As`:

| Type | Sentinel | When |
| --- | --- | --- |
| `*OptionError` | `ErrInvalidOptions` | An option is out of range or unknown. |
| `*CountError` | `ErrParticipantCount` | Too few participants, not a power of two, not a multiple of the group size, or no playable match. |
| `*ParticipantError` | `ErrInvalidParticipant` | Empty, duplicate or reserved participant ID. |
| `*StageError` | (wrapped error) | A stage of a multi-stage plan failed; `Stage` names it. |
| `*LinkError` | `ErrMatchNotFound` | A match or the target of a winner/loser link is missing. |
| (internal) | `ErrInvalidStage` | The stage lacks the data needed to advance. |
| (internal) | `ErrInvalidResult` | `Report` got a result that cannot apply. |
| | `ErrUnknownFormat` | `Registry.Get` with an unregistered key. |

## Performance

Measured with `go test -bench . -benchmem -count 6 -benchtime 500ms`
(Go 1.27.2, linux/amd64, AMD Ryzen 7 5800X3D, pinned to two cores with
`GOMAXPROCS=2`). Cells are the median of 6 runs, ± half the min-max spread.
Scenarios are in [bench_test.go](bench_test.go):

- `Plan/*`: plan the first stage for n participants (`round_robin_group` is
  one round-robin group of n; `ffa_league` uses the defaults).
- `Run/*`: plan, then report every match one at a time with `ReportAt`
  until the stage finishes (Swiss: default rounds, or 12).
- `SwissNextRound/*`: one `Advance` that closes round 6 of 12 and pairs
  round 7.
- `Standings/*`: standings after 6 Swiss rounds.

`Advance` recomputes standings from the recorded results on every call, so
per-report cost grows with the number of matches in the stage; Swiss runs
are dominated by that.

| Benchmark | time/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Plan/single_elimination/n=8 | 795 ns ±6% | 3.72 KiB | 12 |
| Plan/single_elimination/n=64 | 6.65 µs ±2% | 28.6 KiB | 12 |
| Plan/single_elimination/n=512 | 54 µs ±3% | 226 KiB | 12 |
| Plan/single_elimination/n=4096 | 606 µs ±4% | 1.72 MiB | 12 |
| Plan/double_elimination/n=8 | 1.34 µs ±38% | 5.62 KiB | 12 |
| Plan/double_elimination/n=64 | 9.57 µs ±8% | 47.1 KiB | 14 |
| Plan/double_elimination/n=512 | 86.2 µs ±9% | 373 KiB | 14 |
| Plan/double_elimination/n=4096 | 1.06 ms ±11% | 2.85 MiB | 14 |
| Plan/swiss/n=8 | 1.05 µs ±3% | 4.02 KiB | 17 |
| Plan/swiss/n=64 | 8.13 µs ±7% | 28.4 KiB | 17 |
| Plan/swiss/n=512 | 59.6 µs ±21% | 220 KiB | 17 |
| Plan/swiss/n=4096 | 639 µs ±4% | 1.62 MiB | 17 |
| Plan/round_robin_group/n=8 | 2.67 µs ±2% | 12 KiB | 15 |
| Plan/round_robin_group/n=64 | 147 µs ±2% | 754 KiB | 18 |
| Plan/round_robin_group/n=512 | 7.79 ms ±14% | 46.9 MiB | 21 |
| Plan/ffa_league/n=64 | 11.4 µs ±7% | 50.6 KiB | 33 |
| Plan/ffa_league/n=512 | 92.7 µs ±4% | 386 KiB | 174 |
| Plan/ffa_league/n=4096 | 985 µs ±6% | 2.98 MiB | 1967 |
| Run/single_elimination/n=8 | 1.41 µs ±1% | 5.09 KiB | 32 |
| Run/single_elimination/n=64 | 12.4 µs ±6% | 41.3 KiB | 200 |
| Run/single_elimination/n=512 | 101 µs ±2% | 330 KiB | 1544 |
| Run/single_elimination/n=4096 | 1.01 ms ±6% | 2.53 MiB | 12296 |
| Run/double_elimination/n=8 | 2.54 µs ±2% | 7.89 KiB | 44 |
| Run/double_elimination/n=64 | 21.8 µs ±6% | 69 KiB | 326 |
| Run/double_elimination/n=512 | 191 µs ±12% | 552 KiB | 2566 |
| Run/double_elimination/n=4096 | 2.48 ms ±3% | 4.26 MiB | 20486 |
| Run/swiss/n=8 | 20.2 µs ±8% | 37.8 KiB | 205 |
| Run/swiss/n=64 | 2.57 ms ±6% | 4.02 MiB | 3882 |
| Run/swiss/n=512 | 336 ms ±6% | 464 MiB | 56735 |
| Run/swiss_12_rounds/n=64 | 7.34 ms ±2% | 11.1 MiB | 8433 |
| Run/swiss_12_rounds/n=512 | 528 ms ±6% | 751 MiB | 78803 |
| Run/swiss_12_rounds_avoid_rematches/n=64 | 7.35 ms ±3% | 11.1 MiB | 8433 |
| Run/swiss_12_rounds_avoid_rematches/n=512 | 531 ms ±2% | 751 MiB | 78803 |
| SwissNextRound/greedy/n=8 | 4.07 µs ±2% | 6.48 KiB | 31 |
| SwissNextRound/avoid_rematches/n=8 | 4.11 µs ±3% | 6.48 KiB | 31 |
| SwissNextRound/greedy/n=64 | 27 µs ±12% | 51.5 KiB | 38 |
| SwissNextRound/avoid_rematches/n=64 | 33.7 µs ±21% | 51.5 KiB | 38 |
| SwissNextRound/greedy/n=512 | 329 µs ±17% | 411 KiB | 42 |
| SwissNextRound/avoid_rematches/n=512 | 273 µs ±17% | 411 KiB | 42 |
| SwissNextRound/greedy/n=4096 | 2.69 ms ±3% | 3.87 MiB | 64 |
| SwissNextRound/avoid_rematches/n=4096 | 2.68 ms ±3% | 3.87 MiB | 64 |
| Standings/n=8 | 2.61 µs ±1% | 3.52 KiB | 15 |
| Standings/n=64 | 17.6 µs ±3% | 28.3 KiB | 20 |
| Standings/n=512 | 153 µs ±1% | 231 KiB | 24 |
| Standings/n=4096 | 1.93 ms ±2% | 2.52 MiB | 46 |

## License

MIT. See [LICENSE](LICENSE).
