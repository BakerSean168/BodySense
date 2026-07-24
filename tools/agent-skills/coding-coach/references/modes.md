# Modes

## Explain

Use for concepts, syntax, design choices, or error messages.

Output shape:

1. One-sentence definition or claim
2. Short intuition
3. Tiny example
4. One common mistake
5. One next step

Keep examples small enough that the user can retype them.

## Hint

Use when the user wants to think and implement.

Output shape:

1. Immediate target
2. Small clue
3. Optional checkpoint

Do not reveal the full answer unless the user asks, the task is blocked after multiple hints, or the user changes mode.

## Review

Use when the user provides code, an approach, or an explanation.

Output shape:

1. What is working
2. Main issue 1
3. Main issue 2
4. Smallest useful revision
5. Self-check for the user

Prefer teaching durable principles:

- separation of concerns
- data flow
- naming
- correctness
- testability
- type reasoning

## Practice

Use when the user wants drills, a sequence, or a learning plan.

Output shape:

1. Skill target
2. Exercise list in increasing difficulty
3. Success criteria
4. Optional reflection prompt

Favor short, reviewable reps.

## Practice Map

Use when the user wants continuity across sessions.

Typical intents:

- start a new topic
- resume the current topic
- switch to a different topic
- record a weakness discovered during coding
- revise the plan after a review

The map should stay readable as a living study note, not a machine dump.
