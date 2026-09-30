package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/bodysense/api/internal/model"
)

// DiagnosisHistorySnapshotRevision versions the immutable-row hashing contract used
// by the DGS-SAFE-090 rollback rehearsal.
const DiagnosisHistorySnapshotRevision = "diagnosis-history-snapshot-v1"

// DiagnosisHistorySnapshot contains only opaque IDs and hashes. Raw health content
// never leaves the database as part of the operational snapshot.
type DiagnosisHistorySnapshot struct {
	Revision        string            `json:"revision"`
	GeneratedAt     time.Time         `json:"generated_at"`
	AnalysisCount   int               `json:"analysis_count"`
	CandidateCount  int               `json:"candidate_count"`
	AnalysisHashes  map[string]string `json:"analysis_hashes"`
	CandidateHashes map[string]string `json:"candidate_hashes"`
	RootSHA256      string            `json:"root_sha256"`
}

// DiagnosisHistoryVerification proves that every row protected by a pre-rollback
// snapshot still exists with identical immutable content. New rows are allowed.
type DiagnosisHistoryVerification struct {
	Revision                string   `json:"revision"`
	Unchanged               bool     `json:"unchanged"`
	ProtectedAnalysisCount  int      `json:"protected_analysis_count"`
	ProtectedCandidateCount int      `json:"protected_candidate_count"`
	AddedAnalysisCount      int      `json:"added_analysis_count"`
	AddedCandidateCount     int      `json:"added_candidate_count"`
	MissingAnalysisIDs      []string `json:"missing_analysis_ids"`
	MutatedAnalysisIDs      []string `json:"mutated_analysis_ids"`
	MissingCandidateIDs     []string `json:"missing_candidate_ids"`
	MutatedCandidateIDs     []string `json:"mutated_candidate_ids"`
	BeforeRootSHA256        string   `json:"before_root_sha256"`
	ProtectedRootSHA256     string   `json:"protected_root_sha256"`
}

func BuildDiagnosisHistorySnapshot(
	analyses []model.DiagnosisAnalysisRecord,
	candidates []model.DiagnosisCandidateRecord,
	now time.Time,
) (DiagnosisHistorySnapshot, error) {
	analysisHashes := make(map[string]string, len(analyses))
	for _, analysis := range analyses {
		hash, err := hashDiagnosisAnalysis(analysis)
		if err != nil {
			return DiagnosisHistorySnapshot{}, fmt.Errorf("hash diagnosis analysis %s: %w", analysis.ID, err)
		}
		analysisHashes[analysis.ID.String()] = hash
	}
	candidateHashes := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		hash, err := hashDiagnosisCandidate(candidate)
		if err != nil {
			return DiagnosisHistorySnapshot{}, fmt.Errorf("hash diagnosis candidate %s: %w", candidate.ID, err)
		}
		candidateHashes[candidate.ID.String()] = hash
	}
	return diagnosisHistorySnapshotFromHashes(analysisHashes, candidateHashes, now), nil
}

func VerifyDiagnosisHistorySnapshot(before, current DiagnosisHistorySnapshot) DiagnosisHistoryVerification {
	missingAnalyses, mutatedAnalyses := compareProtectedHashes(before.AnalysisHashes, current.AnalysisHashes)
	missingCandidates, mutatedCandidates := compareProtectedHashes(before.CandidateHashes, current.CandidateHashes)
	protectedAnalyses := subsetHashes(current.AnalysisHashes, before.AnalysisHashes)
	protectedCandidates := subsetHashes(current.CandidateHashes, before.CandidateHashes)
	protectedRoot := diagnosisHistoryRoot(protectedAnalyses, protectedCandidates)
	return DiagnosisHistoryVerification{
		Revision:                DiagnosisHistorySnapshotRevision,
		Unchanged:               len(missingAnalyses) == 0 && len(mutatedAnalyses) == 0 && len(missingCandidates) == 0 && len(mutatedCandidates) == 0,
		ProtectedAnalysisCount:  len(before.AnalysisHashes),
		ProtectedCandidateCount: len(before.CandidateHashes),
		AddedAnalysisCount:      addedHashCount(before.AnalysisHashes, current.AnalysisHashes),
		AddedCandidateCount:     addedHashCount(before.CandidateHashes, current.CandidateHashes),
		MissingAnalysisIDs:      missingAnalyses,
		MutatedAnalysisIDs:      mutatedAnalyses,
		MissingCandidateIDs:     missingCandidates,
		MutatedCandidateIDs:     mutatedCandidates,
		BeforeRootSHA256:        before.RootSHA256,
		ProtectedRootSHA256:     protectedRoot,
	}
}

func diagnosisHistorySnapshotFromHashes(analysisHashes, candidateHashes map[string]string, now time.Time) DiagnosisHistorySnapshot {
	return DiagnosisHistorySnapshot{
		Revision:        DiagnosisHistorySnapshotRevision,
		GeneratedAt:     now.UTC(),
		AnalysisCount:   len(analysisHashes),
		CandidateCount:  len(candidateHashes),
		AnalysisHashes:  analysisHashes,
		CandidateHashes: candidateHashes,
		RootSHA256:      diagnosisHistoryRoot(analysisHashes, candidateHashes),
	}
}

func hashDiagnosisAnalysis(analysis model.DiagnosisAnalysisRecord) (string, error) {
	analysis.Candidates = nil
	payload := struct {
		Analysis    model.DiagnosisAnalysisRecord `json:"analysis"`
		ReplayInput json.RawMessage               `json:"replay_input"`
		RawOutput   json.RawMessage               `json:"raw_output"`
	}{
		Analysis:    analysis,
		ReplayInput: canonicalJSON(analysis.ReplayInput),
		RawOutput:   canonicalJSON(analysis.RawOutput),
	}
	return hashCanonicalJSON(payload)
}

func hashDiagnosisCandidate(candidate model.DiagnosisCandidateRecord) (string, error) {
	payload := struct {
		Candidate  model.DiagnosisCandidateRecord `json:"candidate"`
		RawPayload json.RawMessage                `json:"raw_payload"`
	}{
		Candidate:  candidate,
		RawPayload: canonicalJSON(candidate.RawPayload),
	}
	return hashCanonicalJSON(payload)
}

func hashCanonicalJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return "", err
	}
	return sha256Hex(canonical), nil
}

func canonicalJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage(raw)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(raw)
	}
	return encoded
}

func diagnosisHistoryRoot(analysisHashes, candidateHashes map[string]string) string {
	lines := make([]string, 0, len(analysisHashes)+len(candidateHashes))
	for id, hash := range analysisHashes {
		lines = append(lines, "analysis:"+id+":"+hash)
	}
	for id, hash := range candidateHashes {
		lines = append(lines, "candidate:"+id+":"+hash)
	}
	sort.Strings(lines)
	encoded, _ := json.Marshal(lines)
	return sha256Hex(encoded)
}

func compareProtectedHashes(before, current map[string]string) (missing, mutated []string) {
	for id, hash := range before {
		currentHash, ok := current[id]
		if !ok {
			missing = append(missing, id)
			continue
		}
		if currentHash != hash {
			mutated = append(mutated, id)
		}
	}
	sort.Strings(missing)
	sort.Strings(mutated)
	return missing, mutated
}

func subsetHashes(current, protected map[string]string) map[string]string {
	result := make(map[string]string, len(protected))
	for id := range protected {
		if hash, ok := current[id]; ok {
			result[id] = hash
		}
	}
	return result
}

func addedHashCount(before, current map[string]string) int {
	count := 0
	for id := range current {
		if _, ok := before[id]; !ok {
			count++
		}
	}
	return count
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
