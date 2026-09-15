# BS-P7-CONCEPT-SECURITY-HEADERS · browser-enforced HTTP security headers

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-SECURITY-HEADERS**

## Concept

HTTP response security headers provide browser-enforced defense in depth for content execution, framing, MIME handling, referrers and transport.

## Prerequisites

- BS-P7-CONCEPT-XSS
- BS-P3-CONCEPT-SAME-ORIGIN-CORS

## BodySense target files

- `docker/Caddyfile`
- `docker/nginx.conf`
- `scripts/validate-production-proxy.sh`

## Prediction before reading/running

Before reading the proxy config, predict the intended role of CSP, HSTS, X-Content-Type-Options, X-Frame-Options/frame-ancestors and Referrer-Policy, and which browser behavior each should constrain.

## Task

Trace BodySense CSP, HSTS, nosniff, frame and referrer headers from proxy configuration to browser enforcement. Explain which threat each header reduces and why headers complement rather than replace input validation/authz.

## Failure case

Remove or broadly weaken one directive/header (for example CSP script-src or frame protection) and predict the additional attack surface. Do not weaken production configuration just to demonstrate the exercise.

## Verification command / evidence

- Read `docker/Caddyfile`, `docker/nginx.conf` and `scripts/validate-production-proxy.sh` and build a header -> threat -> browser enforcement table.
- Run `bash scripts/validate-production-proxy.sh` or a local response-header probe against a non-production environment if available.

A green proxy validator is evidence about configuration, not mastery. L4 requires explaining what each header does, what it cannot do, and what observation would falsify the expected policy.

## Explain-back questions

- Why does CSP reduce XSS impact but not make unsafe HTML trustworthy?
- How is CORS different from CSP?
- Why are HSTS and X-Content-Type-Options transport/content interpretation controls rather than authentication?

## Production change

No production policy should be weakened for learning. Only change headers when a real compatibility/security requirement is characterized by tests/probes and the least-permissive working policy is understood.

## L4 acceptance

Complete only when the learner can predict header behavior, verify the actual response/configuration, explain defense-in-depth boundaries and identify a concrete falsifying browser/network observation.
