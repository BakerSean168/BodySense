# ADR 0012: Adopt a pinned Posture geometric perception mechanism

- Status: Accepted
- Date: 2026-09-01
- Scope: Posture Agent, MediaPipe pose estimation, immutable configuration identity, durable provenance, deployment images

## Context

BodySense Posture analysis combines two different authorities:

1. a VLM produces qualitative single-view posture observations; and
2. deterministic pose geometry produces the only numeric posture metrics allowed to survive governance.

The second authority is behavior-significant because a geometric metric can become a governed Posture finding and later enter Assessment as visual evidence. The previous `posture-v1` configuration did not fingerprint that behavior. Its immutable ID covered the VLM prompt/schema/model/governance/generation contract, while geometry lived outside the configuration as repository constants and a runtime-downloaded MediaPipe model.

The previous runtime had three concrete drift modes:

```text
posture-v1 configuration id
  -> MediaPipe package may or may not be installed
  -> model URL = .../float16/latest/...
  -> cached model accepted when merely non-empty
  -> geometric thresholds live outside config identity
```

A production audit found an even stronger failure: the AI Docker image installed only the `ocr` optional dependency. `mediapipe` was absent in both Staging and Production, while local/test environments could install the `pose` extra. The same `posture-v1` ID therefore meant VLM-only behavior in deployed environments and potentially VLM + geometry elsewhere.

That violates the intended meaning of immutable behavior identity.

## Decision

### 1. Posture v2 binds geometric perception into the immutable Agent configuration

The current Posture manifest is `posture-v2` and includes a required subordinate non-LLM mechanism:

```yaml
geometry_mechanism:
  required: true
  mechanism_revision: posture-geometry-v1
  engine: mediapipe-tasks
  engine_version: "1.0.0"
  model_uri: https://storage.googleapis.com/mediapipe-models/pose_landmarker/pose_landmarker_lite/float16/1/pose_landmarker_lite.task
  model_sha256: 59929e1d1ee95287735ddd833b19cf4ac46d29bc7afddbbf6753c459690d574a
  threshold_revision: posture-geometry-thresholds-v1
  threshold_sha256: 588917b4a071ee1e249d3930b37769c9c9bd7a4fdebd68eb2a00bfdd13fbb140
```

The entire mechanism object participates in the Posture configuration fingerprint. Changing the engine version, model URI/hash, threshold revision/hash or any other behavior-significant manifest field creates a different `posture-config-*` identity.

Current identity:

```text
posture-v2
posture-config-efa3a84622818772
```

Historical `posture-v1` remains repository-known as:

```text
posture-config-3a774008db422a31
```

Its fingerprint remains byte-compatible because the new optional schema field is excluded when absent. V1 is historical/non-serving; it cannot be selected as the current Posture Champion.

### 2. Geometry is required for current Posture serving

Current v2 does not silently degrade to VLM-only when its declared mechanism is missing or inconsistent.

Before the VLM call, the runtime verifies:

```text
threshold spec SHA256
MediaPipe package version
model file existence
model file SHA256
```

A missing package/model or any identity mismatch fails the Posture request before model generation. A photo with no detectable pose is different: the mechanism can be fully verified yet produce zero geometric findings, in which case qualitative VLM analysis may continue with exact mechanism provenance.

This distinction is intentional:

```text
mechanism unavailable/inconsistent -> contract failure
mechanism verified, no pose found  -> valid zero-metric observation path
```

### 3. The request path never downloads a pose model

The old `latest` runtime download is removed.

Production images provision the exact versioned artifact during Docker build:

```text
/float16/1/pose_landmarker_lite.task
  -> download in builder
  -> verify SHA256
  -> atomic verified artifact
  -> copy into final image
  -> BODYSENSE_POSE_MODEL_PATH=/app/models/pose_landmarker_lite-float16-v1.task
```

Local development provisions the same versioned artifact in an explicit startup preflight before the AI server starts. The request path only verifies and reads an already provisioned model.

### 4. Threshold behavior is a canonical fingerprinted contract

All behavior-significant geometric thresholds are represented in one canonical structure, including:

- landmark visibility threshold;
- C7 proxy offset;
- craniovertebral-angle severity bands;
- shoulder/hip asymmetry bands;
- ear-shoulder offset bands;
- pelvic-tilt proxy bands;
- spine-midline offset bands;
- PoseLandmarker detection/presence/tracking settings.

The canonical JSON SHA256 is:

```text
588917b4a071ee1e249d3930b37769c9c9bd7a4fdebd68eb2a00bfdd13fbb140
```

Runtime recomputes this hash and compares it with the manifest. Editing thresholds without advancing the declared mechanism identity therefore fails closed instead of silently changing the meaning of an existing Posture configuration.

### 5. Exact mechanism provenance is durable Posture evidence

Python attaches `mechanism_provenance` to the governed Posture result:

```text
status = verified
mechanism_revision
engine
engine_version
model_uri
model_sha256
threshold_revision
threshold_sha256
```

Go does not trust that envelope merely because the Agent configuration ID matches. Before persisting `user_uploads.analysis_result`, Go independently compares every mechanism field with its repository-known registration for the selected Posture configuration.

Missing provenance or any mismatch fails closed.

The Go `generation_decision_trace` also records the mechanism revision/model hash/threshold identity used for the durable persistence decision.

### 6. Deployment images must prove the mechanism, not merely declare it

The AI image installs the `pose` extra in addition to OCR dependencies. Production-shaped validation executes `verify_pose_mechanism()` inside the built/running AI container and requires the exact current Posture configuration ID and verified mechanism identity before continuing to migration/E2E validation.

This specifically prevents a recurrence of the previous drift where source code declared geometry but the deployed image omitted MediaPipe.

## Persistence and compatibility

No relational migration is required.

`user_uploads.analysis_result` is already JSONB and stores the governed Posture result, so current v2 results carry `mechanism_provenance` inside the existing durable payload. `user_uploads.agent_configuration_id` continues to store the exact Posture configuration ID.

Historical v1 rows retain their original payloads and configuration identity. No backfill invents mechanism provenance for analyses that never recorded it.

## Durable invariant

```text
current Posture config
  -> exact pinned geometry mechanism exists
  -> runtime verifies engine + model + threshold identity
  -> geometry executes
  -> governed result carries exact mechanism provenance
  -> Go independently revalidates provenance
  -> only then persist analysis_result
```

And:

```text
same posture-config id => same declared VLM + geometric behavior contract
```

## Consequences

### Positive

- Staging/Production cannot silently become VLM-only while claiming the current Posture configuration;
- no runtime dependency on an upstream `latest` model pointer;
- corrupt or replaced pose-model bytes are detected before analysis;
- threshold drift is part of immutable behavior identity;
- every current durable geometric Posture result explains the exact engine/model/threshold mechanism that produced it;
- Assessment receives visual evidence whose upstream perception identity is auditable.

### Trade-offs

- AI image size increases because MediaPipe/OpenCV/Numpy are now required runtime dependencies;
- Docker build performs one versioned external artifact acquisition, guarded by SHA256;
- current Posture analysis fails if the required mechanism is unavailable rather than silently degrading to VLM-only;
- changing geometric thresholds/model/package version now requires an explicit Posture configuration revision.

## Rejected alternatives

### Keep geometry optional and record `available=false`

Rejected for the current serving contract. Numeric geometry is deliberately authoritative and behavior-significant; allowing environment-dependent omission under one immutable config reproduces the original drift.

### Keep the `latest` model URL but record its runtime hash

Rejected because replay/audit would still depend on mutable upstream selection and a fresh deployment could fetch different behavior under the same source revision.

### Trust a non-empty cached model

Rejected because file existence does not prove artifact identity.

### Record mechanism provenance only in Python

Rejected because Python output crosses a trust boundary before durable Go persistence. Go must independently compare the returned mechanism identity with repository-known configuration.

### Put only a `threshold_revision` label in the manifest

Rejected because a developer could change constants without bumping the label. The canonical threshold hash makes the declaration executable and fail-closed.
