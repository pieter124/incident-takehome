package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"
)

type Schedule struct {
	Users                []string  `json:"users"`
	HandoverStartAt      time.Time `json:"handover_start_at"`
	HandoverIntervalDays int       `json:"handover_interval_days"`
}

type Entry struct {
	User    string    `json:"user"`
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}

type Entries []Entry

// ValidateWindow rejects a render window that doesn't make sense. from == until
// is rejected too, a zero-length window can never contain a valid entry (see
// notes.md).
func ValidateWindow(from, until time.Time) error {
	if !from.Before(until) {
		return fmt.Errorf("--from must be before --until")
	}
	return nil
}

// Validate checks that a parsed Schedule is complete enough to generate a
// rotation from. It doesn't care about from/until, only about the schedule's
// own shape.
func (s Schedule) Validate() error {
	if len(s.Users) == 0 {
		return fmt.Errorf("schedule must have at least one user")
	}
	if s.HandoverIntervalDays <= 0 {
		return fmt.Errorf("handover_interval_days must be a positive integer")
	}
	if s.HandoverStartAt.IsZero() {
		return fmt.Errorf("handover_start_at is required")
	}
	return nil
}

// GenerateEntries returns the full base rotation from handover_start_at up to
// until, one entry per shift. It only depends on until, not from: clipping the
// front of the list down to from happens later, as a separate step, once
// overrides have already been applied.
//
// A plain loop, rather than jumping straight to the shift covering some start
// time. Decided against the jump in notes.md: even a schedule starting at the
// unix epoch only means tens of thousands of loop iterations, still instant,
// so there's no real performance reason to make this harder to read.
func (s Schedule) GenerateEntries(until time.Time) Entries {
	interval := time.Duration(s.HandoverIntervalDays) * 24 * time.Hour

	var entries Entries
	start := s.HandoverStartAt
	for i := 0; start.Before(until); i++ {
		end := start.Add(interval)
		entries = append(entries, Entry{
			User:    s.Users[i%len(s.Users)],
			StartAt: start,
			EndAt:   end,
		})
		start = end
	}

	return entries
}

func byStartAt(a, b Entry) int {
	return a.StartAt.Compare(b.StartAt)
}

// carveOverride splits entry around override's range, returning whatever is
// left of entry once override has taken over whatever portion they share.
// Returns entry unchanged if they don't overlap at all, nothing if override
// fully consumes it, or up to two pieces if override only covers the start,
// the end, or the middle of entry.
func carveOverride(entry, override Entry) Entries {
	if !entry.EndAt.After(override.StartAt) || !entry.StartAt.Before(override.EndAt) {
		return Entries{entry}
	}

	var remaining Entries
	if entry.StartAt.Before(override.StartAt) {
		remaining = append(remaining, Entry{
			User:    entry.User,
			StartAt: entry.StartAt,
			EndAt:   override.StartAt,
		})
	}
	if entry.EndAt.After(override.EndAt) {
		remaining = append(remaining, Entry{
			User:    entry.User,
			StartAt: override.EndAt,
			EndAt:   entry.EndAt,
		})
	}
	return remaining
}

// ApplyOverrides carves each override's window out of e and inserts the
// override in its place. Sorts both e and overrides itself, by start time, so
// neither has to already be sorted by the caller.
//
// Precondition: overrides must be pairwise non-overlapping, which the brief
// guarantees. That's what makes a single pass over them safe, once an
// override has been spliced in, no later override in the loop can overlap
// it, so it passes through carveOverride unchanged on every later iteration.
// Only the original entries in e ever get split.
func (e Entries) ApplyOverrides(overrides Entries) Entries {
	slices.SortFunc(e, byStartAt)
	slices.SortFunc(overrides, byStartAt)

	result := e
	for _, override := range overrides {
		var withGap Entries
		for _, entry := range result {
			withGap = append(withGap, carveOverride(entry, override)...)
		}
		result = append(withGap, override)
	}

	slices.SortFunc(result, byStartAt)
	return result
}

// Truncate clips e down to the window [from, until). Entries entirely
// outside the window are dropped, and whichever entries overlap the edges
// have their start or end clipped. It doesn't care whether an entry came
// from the base rotation or from an override, it only looks at StartAt and
// EndAt, so it's reused for both the final render window and for clipping
// an already-loaded overrides file to that same window.
func (e Entries) Truncate(from, until time.Time) Entries {
	var truncated Entries
	for _, entry := range e {
		if !entry.EndAt.After(from) || !entry.StartAt.Before(until) {
			continue
		}
		if entry.StartAt.Before(from) {
			entry.StartAt = from
		}
		if entry.EndAt.After(until) {
			entry.EndAt = until
		}
		truncated = append(truncated, entry)
	}
	return truncated
}

// Merge combines consecutive entries in e that belong to the same user into
// one entry. Assumes e is already sorted.
//
// Only merges entries that are both the same user and already back to back
// in time (prev.EndAt == next.StartAt), rather than merging on a matching
// user alone, so it can never silently paper over a gap if it's ever fed
// one.
func (e Entries) Merge() Entries {
	var merged Entries
	for _, entry := range e {
		if last := len(merged) - 1; last >= 0 && merged[last].User == entry.User && merged[last].EndAt.Equal(entry.StartAt) {
			merged[last].EndAt = entry.EndAt
			continue
		}
		merged = append(merged, entry)
	}
	return merged
}

// BuildSchedule wires the pipeline together: validate, generate the base
// rotation, apply overrides, truncate to the window, then merge consecutive
// same-user entries. from is clamped to schedule.HandoverStartAt first,
// since nobody is on call before the schedule starts.
func BuildSchedule(schedule Schedule, overrides Entries, from, until time.Time) (Entries, error) {
	if err := ValidateWindow(from, until); err != nil {
		return nil, err
	}
	if err := schedule.Validate(); err != nil {
		return nil, err
	}

	effectiveFrom := from
	if schedule.HandoverStartAt.After(effectiveFrom) {
		effectiveFrom = schedule.HandoverStartAt
	}

	entries := schedule.GenerateEntries(until)
	entries = entries.ApplyOverrides(overrides)
	entries = entries.Truncate(effectiveFrom, until)
	entries = entries.Merge()

	return entries, nil
}

// LoadSchedule reads and parses a schedule JSON file. It doesn't call
// Validate, that's a separate step, so file/parse errors and semantic
// validation errors stay distinguishable.
func LoadSchedule(path string) (Schedule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Schedule{}, fmt.Errorf("reading schedule file: %w", err)
	}

	var schedule Schedule
	if err := json.Unmarshal(data, &schedule); err != nil {
		return Schedule{}, fmt.Errorf("parsing schedule file: %w", err)
	}

	return schedule, nil
}

// LoadOverrides reads and parses an overrides JSON file. Whether to call this
// at all, based on whether --overrides was passed, is a CLI-layer decision
// made elsewhere.
func LoadOverrides(path string) (Entries, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading overrides file: %w", err)
	}

	var overrides Entries
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("parsing overrides file: %w", err)
	}

	return overrides, nil
}
