# Hint Ladder

Use the lowest rung that will keep the user moving.

## Rung 1: Goal

State the specific sub-problem the user should solve next.

Example:
`Focus only on deriving the filtered array before you worry about rendering it.`

## Rung 2: Pointer

Point to the concept, API, or file that matters.

Example:
`You likely need to look at how `map` differs from `filter` here.`

## Rung 3: Checkpoint

Suggest a small validation step.

Example:
`Log the value right before the return and check whether it is an array or `undefined`.`

## Rung 4: Shape

Provide pseudocode, an outline, or a very small code fragment.

Example:
```ts
const nextValue = items.???(...)
return nextValue
```

## Rung 5: Answer

Provide the direct implementation only when one of these is true:

- the user asks for the answer
- the user has made a serious attempt and is still blocked
- the task is no longer about learning and has shifted to delivery

When giving the answer, still call out the key reasoning so the user does not get a paste-only response.
