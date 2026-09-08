# BS-P7-CONCEPT-DEPENDENCY-SECURITY · dependency and software-supply-chain security

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-DEPENDENCY-SECURITY**

## Concept

Dependency auditing, lockfiles and supply-chain controls reduce the risk of vulnerable, compromised or unexpectedly changed third-party code.

## Prerequisites

- BS-P11-CONCEPT-REPRODUCIBLE-PIPELINE

## BodySense target files

- `pnpm-lock.yaml`
- `package.json`
- `.github/workflows/ci.yml`

## Prediction before reading/running

Predict which mechanisms pin dependency identity/integrity in local/CI installs and which threats remain possible even with a lockfile.

## Task

Explain lockfile/CI install guarantees, run the repository-appropriate dependency audit when network access permits, and classify actionable versus non-actionable findings. Include install-script, typosquatting/maintainer compromise and breaking-upgrade trade-offs.

## Failure case

Treat `audit fix --force` or a mass upgrade as automatically safe, or assume a lockfile protects against a dependency version that was already malicious when pinned.

## Verification command / evidence

- Trace the package-manager/lockfile and CI install path; identify whether installs are frozen/reproducible.
- Run `pnpm audit --prod` when registry access is available, then classify findings by reachability/severity/fix risk rather than blindly applying every suggested change.

A clean audit is not proof of supply-chain safety. L4 requires explaining the trust model, transitive/install-time attack surface and what controls detect/prevent which failure modes.

## Explain-back questions

- What does a lockfile guarantee and what does it not?
- Why can install scripts be especially dangerous?
- Why can a security upgrade itself require regression validation?

## Production change

Do not mass-upgrade dependencies for the exercise. Apply a dependency change only with a concrete advisory/maintenance reason plus compatibility tests and rollback awareness.

## L4 acceptance

Complete only when the learner can map dependency threats to controls, inspect the actual BodySense install/CI policy, interpret audit evidence and explain a falsifying/remaining-risk scenario.
