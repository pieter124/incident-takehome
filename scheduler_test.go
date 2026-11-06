package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestValidateWindow(t *testing.T) {
	from := time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC)
	until := time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)

	tests := map[string]struct {
		from, until time.Time
		wantErr     bool
	}{
		"from before until is valid":   {from: from, until: until, wantErr: false},
		"from after until is rejected": {from: until, until: from, wantErr: true},
		// Decided in notes.md: from == until is rejected the same way as from > until,
		// rather than being treated as a valid empty-array result.
		"from equal to until is rejected": {from: from, until: from, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateWindow(tc.from, tc.until)
			if tc.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestScheduleValidate(t *testing.T) {
	tests := map[string]struct {
		schedule Schedule
		wantErr  bool
	}{
		"valid schedule": {
			schedule: Schedule{
				Users:                []string{"alice", "bob"},
				HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
				HandoverIntervalDays: 7,
			},
			wantErr: false,
		},
		"no users": {
			schedule: Schedule{
				Users:                nil,
				HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
				HandoverIntervalDays: 7,
			},
			wantErr: true,
		},
		"zero handover interval": {
			schedule: Schedule{
				Users:                []string{"alice", "bob"},
				HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
				HandoverIntervalDays: 0,
			},
			wantErr: true,
		},
		"negative handover interval": {
			schedule: Schedule{
				Users:                []string{"alice", "bob"},
				HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
				HandoverIntervalDays: -1,
			},
			wantErr: true,
		},
		"missing handover_start_at": {
			schedule: Schedule{
				Users:                []string{"alice", "bob"},
				HandoverStartAt:      time.Time{},
				HandoverIntervalDays: 7,
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.schedule.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestGenerateEntries(t *testing.T) {
	tests := map[string]struct {
		schedule Schedule
		until    time.Time
		want     Entries
	}{
		"single user repeats every rotation": {
			schedule: Schedule{
				Users:                []string{"alice"},
				HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
				HandoverIntervalDays: 7,
			},
			until: time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC),
			want: Entries{
				{User: "alice", StartAt: time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)},
				{User: "alice", StartAt: time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC)},
				{User: "alice", StartAt: time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC)},
			},
		},
		"multiple users wrap around after reaching the end of the list": {
			schedule: Schedule{
				Users:                []string{"alice", "bob", "charlie"},
				HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
				HandoverIntervalDays: 7,
			},
			until: time.Date(2026, 12, 4, 17, 0, 0, 0, time.UTC),
			want: Entries{
				{User: "alice", StartAt: time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)},
				{User: "bob", StartAt: time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC)},
				{User: "charlie", StartAt: time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC)},
				{User: "alice", StartAt: time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 12, 4, 17, 0, 0, 0, time.UTC)},
			},
		},
		"until lands exactly on a handover boundary, that next shift is not generated": {
			schedule: Schedule{
				Users:                []string{"alice", "bob", "charlie"},
				HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
				HandoverIntervalDays: 7,
			},
			until: time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC),
			want: Entries{
				{User: "alice", StartAt: time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)},
				{User: "bob", StartAt: time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC)},
			},
		},
		"one day interval": {
			schedule: Schedule{
				Users:                []string{"alice", "bob"},
				HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
				HandoverIntervalDays: 1,
			},
			until: time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC),
			want: Entries{
				{User: "alice", StartAt: time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 7, 17, 0, 0, 0, time.UTC)},
				{User: "bob", StartAt: time.Date(2026, 11, 7, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 8, 17, 0, 0, 0, time.UTC)},
				{User: "alice", StartAt: time.Date(2026, 11, 8, 17, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC)},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.schedule.GenerateEntries(tc.until)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestApplyOverrides(t *testing.T) {
	// Shared reference points, reused across cases below. The handover
	// boundaries of a 3 user, 7 day schedule starting 2026-11-06T17:00:00Z.
	t0 := time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC)

	tests := map[string]struct {
		entries   Entries
		overrides Entries
		want      Entries
	}{
		"no overrides returns the entries unchanged": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
			},
			overrides: nil,
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
			},
		},
		"override fully inside one entry splits it into three": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t3},
			},
			overrides: Entries{
				{User: "charlie", StartAt: t1, EndAt: t2},
			},
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "charlie", StartAt: t1, EndAt: t2},
				{User: "alice", StartAt: t2, EndAt: t3},
			},
		},
		"override starts exactly at the entry start and ends partway through": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t3},
			},
			overrides: Entries{
				{User: "charlie", StartAt: t0, EndAt: t2},
			},
			want: Entries{
				{User: "charlie", StartAt: t0, EndAt: t2},
				{User: "alice", StartAt: t2, EndAt: t3},
			},
		},
		"override starts partway through and ends exactly at the entry end": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t3},
			},
			overrides: Entries{
				{User: "charlie", StartAt: t1, EndAt: t3},
			},
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "charlie", StartAt: t1, EndAt: t3},
			},
		},
		"override exactly matches one entry's start and end, replacing it entirely": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
			},
			overrides: Entries{
				{User: "charlie", StartAt: t1, EndAt: t2},
			},
			// bob's whole shift is gone, replaced by charlie. This leaves two
			// adjacent charlie entries, merging them is a separate later step,
			// not something ApplyOverrides does itself.
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "charlie", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
			},
		},
		"override spans a handover boundary, starting in one entry and ending in the next": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
			},
			overrides: Entries{
				{
					User:    "charlie",
					StartAt: time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC),
					EndAt:   time.Date(2026, 11, 14, 17, 0, 0, 0, time.UTC),
				},
			},
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC)},
				{
					User:    "charlie",
					StartAt: time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC),
					EndAt:   time.Date(2026, 11, 14, 17, 0, 0, 0, time.UTC),
				},
				{User: "bob", StartAt: time.Date(2026, 11, 14, 17, 0, 0, 0, time.UTC), EndAt: t2},
			},
		},
		"override exactly covers two full entries, consuming them entirely": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
			},
			overrides: Entries{
				{User: "dave", StartAt: t0, EndAt: t2},
			},
			want: Entries{
				{User: "dave", StartAt: t0, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
			},
		},
		"a user can override their own shift, even though it looks like a no-op": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t3},
			},
			overrides: Entries{
				{User: "alice", StartAt: t1, EndAt: t2},
			},
			// Still splits into three adjacent alice entries, same as any other
			// override. Not special cased, and not merged back here.
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "alice", StartAt: t1, EndAt: t2},
				{User: "alice", StartAt: t2, EndAt: t3},
			},
		},
		"multiple non-overlapping overrides, given out of chronological order": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
			},
			// Listed with the later override first, to check ApplyOverrides
			// sorts its own input rather than relying on the caller to.
			overrides: Entries{
				{
					User:    "eve",
					StartAt: time.Date(2026, 11, 22, 17, 0, 0, 0, time.UTC),
					EndAt:   time.Date(2026, 11, 24, 17, 0, 0, 0, time.UTC),
				},
				{
					User:    "dave",
					StartAt: time.Date(2026, 11, 8, 17, 0, 0, 0, time.UTC),
					EndAt:   time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC),
				},
			},
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: time.Date(2026, 11, 8, 17, 0, 0, 0, time.UTC)},
				{
					User:    "dave",
					StartAt: time.Date(2026, 11, 8, 17, 0, 0, 0, time.UTC),
					EndAt:   time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC),
				},
				{User: "alice", StartAt: time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC), EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: time.Date(2026, 11, 22, 17, 0, 0, 0, time.UTC)},
				{
					User:    "eve",
					StartAt: time.Date(2026, 11, 22, 17, 0, 0, 0, time.UTC),
					EndAt:   time.Date(2026, 11, 24, 17, 0, 0, 0, time.UTC),
				},
				{User: "charlie", StartAt: time.Date(2026, 11, 24, 17, 0, 0, 0, time.UTC), EndAt: t3},
			},
		},
		"override that doesn't overlap any entry is still inserted in order": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
			},
			overrides: Entries{
				{User: "bob", StartAt: t2, EndAt: t3},
			},
			// Nothing here clips this down to the render window, that's the
			// generic truncate step's job, done separately after this.
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t2, EndAt: t3},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.entries.ApplyOverrides(tc.overrides)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC)
	t5 := time.Date(2026, 12, 4, 17, 0, 0, 0, time.UTC)

	tests := map[string]struct {
		entries     Entries
		from, until time.Time
		want        Entries
	}{
		"drops an entry ending exactly at from and one starting exactly at until, keeps the rest untouched": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
				{User: "dave", StartAt: t3, EndAt: t4},
				{User: "eve", StartAt: t4, EndAt: t5},
			},
			from:  t1,
			until: t4,
			want: Entries{
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
				{User: "dave", StartAt: t3, EndAt: t4},
			},
		},
		"clips the first entry's start to from": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t4},
			},
			from:  t2,
			until: t4,
			want: Entries{
				{User: "alice", StartAt: t2, EndAt: t4},
			},
		},
		"clips the last entry's end to until": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t4},
			},
			from:  t0,
			until: t2,
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t2},
			},
		},
		"clips both the start and the end when a single entry spans the whole window": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t5},
			},
			from:  t1,
			until: t3,
			want: Entries{
				{User: "alice", StartAt: t1, EndAt: t3},
			},
		},
		"clips the first and last entries while leaving the one in between untouched": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t2},
				{User: "bob", StartAt: t2, EndAt: t3},
				{User: "charlie", StartAt: t3, EndAt: t5},
			},
			from:  t1,
			until: t4,
			want: Entries{
				{User: "alice", StartAt: t1, EndAt: t2},
				{User: "bob", StartAt: t2, EndAt: t3},
				{User: "charlie", StartAt: t3, EndAt: t4},
			},
		},
		"every entry is outside the window, nothing survives": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t4, EndAt: t5},
			},
			from:  t2,
			until: t3,
			want:  nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.entries.Truncate(tc.from, tc.until)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	t0 := time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 12, 4, 17, 0, 0, 0, time.UTC)
	t5 := time.Date(2026, 12, 11, 17, 0, 0, 0, time.UTC)

	tests := map[string]struct {
		entries Entries
		want    Entries
	}{
		"different users throughout are left unchanged": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
			},
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "bob", StartAt: t1, EndAt: t2},
				{User: "charlie", StartAt: t2, EndAt: t3},
			},
		},
		"two consecutive entries with the same user are merged into one": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "alice", StartAt: t1, EndAt: t2},
			},
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t2},
			},
		},
		"three consecutive entries with the same user are merged into one spanning all three": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "alice", StartAt: t1, EndAt: t2},
				{User: "alice", StartAt: t2, EndAt: t3},
			},
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t3},
			},
		},
		"only adjacent entries with the same user merge, the rest stay separate": {
			entries: Entries{
				{User: "alice", StartAt: t0, EndAt: t1},
				{User: "alice", StartAt: t1, EndAt: t2},
				{User: "bob", StartAt: t2, EndAt: t3},
				{User: "charlie", StartAt: t3, EndAt: t4},
				{User: "charlie", StartAt: t4, EndAt: t5},
			},
			want: Entries{
				{User: "alice", StartAt: t0, EndAt: t2},
				{User: "bob", StartAt: t2, EndAt: t3},
				{User: "charlie", StartAt: t3, EndAt: t5},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.entries.Merge()
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestBuildSchedule is the README's worked example, run end to end through
// the whole pipeline. Every step already has its own focused tests above,
// this one just checks they're wired together correctly.
func TestBuildSchedule(t *testing.T) {
	schedule := Schedule{
		Users:                []string{"alice", "bob", "charlie"},
		HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
		HandoverIntervalDays: 7,
	}
	overrides := Entries{
		{
			User:    "charlie",
			StartAt: time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC),
		},
	}
	from := time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC)
	until := time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC)

	want := Entries{
		{
			User:    "alice",
			StartAt: time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC),
		},
		{
			User:    "charlie",
			StartAt: time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC),
		},
		{
			User:    "alice",
			StartAt: time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC),
		},
		{
			User:    "bob",
			StartAt: time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC),
		},
	}

	got, err := BuildSchedule(schedule, overrides, from, until)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLoadSchedule(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := LoadSchedule(filepath.Join(t.TempDir(), "does-not-exist.json"))
		if err == nil {
			t.Fatal("expected an error for a missing file, got nil")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "schedule.json")
		if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
			t.Fatalf("failed to write test fixture: %v", err)
		}

		_, err := LoadSchedule(path)
		if err == nil {
			t.Fatal("expected an error for invalid json, got nil")
		}
	})

	t.Run("valid schedule", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "schedule.json")
		contents := `{
			"users": ["alice", "bob", "charlie"],
			"handover_start_at": "2026-11-06T17:00:00Z",
			"handover_interval_days": 7
		}`
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatalf("failed to write test fixture: %v", err)
		}

		got, err := LoadSchedule(path)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		want := Schedule{
			Users:                []string{"alice", "bob", "charlie"},
			HandoverStartAt:      time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC),
			HandoverIntervalDays: 7,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})
}

func TestLoadOverrides(t *testing.T) {
	// Not provided at all isn't tested here, that's a CLI-layer decision (whether
	// to call LoadOverrides at all based on the --overrides flag), covered in the
	// CLI phase of the plan, not here.

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadOverrides(filepath.Join(t.TempDir(), "does-not-exist.json"))
		if err == nil {
			t.Fatal("expected an error for a missing file, got nil")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "overrides.json")
		if err := os.WriteFile(path, []byte("not json at all"), 0644); err != nil {
			t.Fatalf("failed to write test fixture: %v", err)
		}

		_, err := LoadOverrides(path)
		if err == nil {
			t.Fatal("expected an error for invalid json, got nil")
		}
	})

	t.Run("wrong shape, an object instead of an array", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "overrides.json")
		if err := os.WriteFile(path, []byte(`{"user": "alice"}`), 0644); err != nil {
			t.Fatalf("failed to write test fixture: %v", err)
		}

		_, err := LoadOverrides(path)
		if err == nil {
			t.Fatal("expected an error, overrides must be a json array")
		}
	})

	t.Run("valid overrides", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "overrides.json")
		contents := `[
			{"user": "charlie", "start_at": "2026-11-09T17:00:00Z", "end_at": "2026-11-09T22:00:00Z"}
		]`
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatalf("failed to write test fixture: %v", err)
		}

		got, err := LoadOverrides(path)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		want := Entries{
			{
				User:    "charlie",
				StartAt: time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC),
				EndAt:   time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC),
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})
}
