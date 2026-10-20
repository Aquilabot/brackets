# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-10-20

### Added

- `Format` strategy interface (`Key`, `Validate`, `Plan`, `Advance`) and a
  `Registry` keyed by format name, with `DefaultRegistry` for the built-ins.
- Formats: `single_elimination`, `double_elimination` (upper/lower bracket,
  bottom half of the seeds starting in the lower bracket), `swiss`,
  `round_robin`, `group_stage` (round robin groups or free-for-all lobbies),
  `ffa_league` and `multi_stage` (chained group stages and matchpoint
  finals fed by placeholders).
- Standard bracket seeding (`SeedOrder`), byes for the top seeds, phantom
  match resolution.
- Swiss pairing with `AvoidRematches` (backtracking, rematch only when
  unavoidable) and `Greedy` strategies; optional deterministic shuffling of
  tied records from a persisted seed or an explicit `rand.Source`.
- `Standings` and `GroupStandings` with documented tie-breakers.
- `Report` / `ReportAt` helpers and `Stage.Apply`.
- `Tally` (with `Swiss.AdvanceTally`), `SingleElimination.Parent` and
  `DoubleElimination.Moves` for callers that keep matches in their own
  storage.
- `Qualifiers` and `FillPlaceholders` to chain stages.
- Typed errors (`OptionError`, `CountError`, `ParticipantError`,
  `StageError`, `LinkError`) wrapping sentinel errors.
- Unit tests, fuzz tests for format invariants, benchmarks.
