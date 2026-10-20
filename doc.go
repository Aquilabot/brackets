// Package brackets generates and runs tournament formats: single and
// double elimination brackets, Swiss, round robin, group stages,
// free-for-all lobby leagues and multi-stage flows. It uses only the Go
// standard library.
//
// Give a [Format] participants and [Options]; it returns [Stage] values
// holding a deterministic schedule of [Match] values. When a match
// finishes, record the result on the stage and call [Format.Advance] (or
// use [Report], which validates the result, advances and applies the
// outcome in one call). Formats are stateless: the stage is plain data you
// can persist between calls, and every call recomputes what it needs from
// the stage, so corrected results are picked up on the next call. Callers
// that keep matches in their own storage can feed them to a [Tally] one at
// a time instead of building Stage.Matches.
//
// Formats are registered by key in a [Registry]; [DefaultRegistry] holds
// every built-in format.
package brackets
