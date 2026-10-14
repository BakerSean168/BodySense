package model

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func episodeSignal(claim, when string, revision int64) EpisodeStateBasis {
	temporal := FactTemporal{Precision: "unknown"}
	if when != "" {
		temporal = FactTemporal{Precision: "day", Start: when}
	}
	return EpisodeStateBasis{
		FactID: uuid.New(), Claim: claim, AssertionStatus: "asserted",
		SourceAvailability: "available", AsOf: temporal, CreatedRevision: revision,
	}
}

func TestEpisodeNoImplicitRecoveryOrResolution(t *testing.T) {
	projection, err := ProjectExplicitEpisodeState("current", 1, nil)
	if err != nil || projection.State != "unknown" || projection.Reason != "insufficient_evidence" || len(projection.BasisFactIDs) != 0 {
		t.Fatalf("bare symptom or silence cannot decide course: %#v %v", projection, err)
	}
	if projection.StateAsOf != "" {
		t.Fatal("unknown status cannot invent event time")
	}
	// An explicit negative symptom Fact alone is not a resolution command.
	// It is not given a status claim and therefore cannot end an Episode.
	projection, err = ProjectExplicitEpisodeState("invalidated", 9, []EpisodeStateBasis{episodeSignal("explicit_resolution", "2026-10-09", 4)})
	if err != nil || projection.State != "unknown" {
		t.Fatalf("invalidated episode cannot stay ended: %#v %v", projection, err)
	}
}

func TestEpisodeExplicitOnsetAndLaterResolution(t *testing.T) {
	start := episodeSignal("explicit_current_report", "2026-10-01", 1)
	stop := episodeSignal("explicit_resolution", "2026-10-04", 2)
	state, err := ProjectExplicitEpisodeState("current", 2, []EpisodeStateBasis{stop, start})
	if err != nil || state.State != "ended" || state.Reason != "explicit_resolution" ||
		state.StateAsOf != "2026-10-04" || state.BasedOnRevision != 2 || len(state.BasisFactIDs) != 2 ||
		state.BasisFactIDs[0] != start.FactID {
		t.Fatalf("valid explicit course: %#v %v", state, err)
	}
	state, err = ProjectExplicitEpisodeState("current", 1, []EpisodeStateBasis{start, stop})
	if err != nil || state.State != "ongoing" || state.BasedOnRevision != 1 || len(state.BasisFactIDs) != 1 {
		t.Fatalf("past revision must ignore future report: %#v %v", state, err)
	}
}

func TestEpisodeRecurrenceRequiresNewIdentity(t *testing.T) {
	resolved := episodeSignal("explicit_resolution", "2026-10-02", 2)
	repeated := episodeSignal("explicit_current_report", "2026-10-05", 3)
	state, err := ProjectExplicitEpisodeState("current", 3, []EpisodeStateBasis{resolved, repeated})
	if err != nil || state.State != "unknown" || state.Reason != "conflicting_evidence" || state.StateAsOf != "" {
		t.Fatalf("must not reopen ended episode: %#v %v", state, err)
	}
}

func TestEpisodeConflictsDoNotGuessChronology(t *testing.T) {
	cases := []struct {
		name  string
		after FactTemporal
	}{
		{"unknown", FactTemporal{Precision: "unknown"}},
		{"approximate", FactTemporal{Precision: "approximate", Start: "2026-10-04"}},
		{"same-day", FactTemporal{Precision: "day", Start: "2026-10-01"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := episodeSignal("explicit_current_report", "2026-10-01", 1)
			b := episodeSignal("explicit_resolution", "", 2)
			b.AsOf = tc.after
			state, err := ProjectExplicitEpisodeState("current", 2, []EpisodeStateBasis{a, b})
			if err != nil || state.State != "unknown" || state.Reason != "conflicting_evidence" {
				t.Fatalf("unorderable evidence guessed status: %#v %v", state, err)
			}
		})
	}
}

func TestEpisodeWithdrawnOrBlockedEvidenceCannotDriveState(t *testing.T) {
	active := episodeSignal("explicit_current_report", "2026-10-01", 1)
	cleared := episodeSignal("explicit_resolution", "2026-10-03", 2)
	cleared.AssertionStatus = "withdrawn"
	state, err := ProjectExplicitEpisodeState("current", 2, []EpisodeStateBasis{active, cleared})
	if err != nil || state.State != "ongoing" || len(state.BasisFactIDs) != 1 {
		t.Fatalf("withdrawn resolution must not be retained: %#v %v", state, err)
	}
	active.SourceAvailability = "erased"
	state, err = ProjectExplicitEpisodeState("current", 2, []EpisodeStateBasis{active, cleared})
	if err != nil || state.State != "unknown" {
		t.Fatalf("erased source must not support projection: %#v %v", state, err)
	}
}

func TestEpisodeProjectionRejectsInvalidStatusClaims(t *testing.T) {
	invalid := episodeSignal("inferred_resolution", "2026-10-04", 1)
	if _, err := ProjectExplicitEpisodeState("current", 1, []EpisodeStateBasis{invalid}); !errors.Is(err, ErrInvalidEpisodeEvidence) {
		t.Fatalf("unsupported claim should fail closed: %v", err)
	}
	invalid = episodeSignal("explicit_current_report", "2026-10-04", 1)
	invalid.AsOf = FactTemporal{Precision: "unknown", Start: "2026-10-04"}
	if _, err := ProjectExplicitEpisodeState("current", 1, []EpisodeStateBasis{invalid}); !errors.Is(err, ErrInvalidEpisodeEvidence) {
		t.Fatalf("invented unknown-time boundary: %v", err)
	}
	if _, err := ProjectExplicitEpisodeState("clinically_diagnosed", 1, nil); !errors.Is(err, ErrInvalidEpisodeEvidence) {
		t.Fatalf("a medical diagnosis is not an episode identity status: %v", err)
	}
}

func TestEpisodeStatusAsOfIsNotRecordTimestamp(t *testing.T) {
	source := episodeSignal("explicit_resolution", "", 2)
	state, err := ProjectExplicitEpisodeState("current", 2, []EpisodeStateBasis{source})
	if err != nil || state.State != "ended" || state.StateAsOf != "" {
		t.Fatalf("unknown occurrence time must remain unknown: %#v %v", state, err)
	}
}
