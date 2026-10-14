// Specification-only oracle for BodySense V2 PC-D6/PC-D9 retention.
// This module does not inspect PostgreSQL, runtime checkpoints, or controlled copies.
// Evidence flags and receipts are caller-supplied *test fixtures*, never real authorization.

export class RetentionContractError extends Error {
  constructor(code, message) {
    super(message);
    this.name = "RetentionContractError";
    this.code = code;
  }
}

function reject(code, message) {
  throw new RetentionContractError(code, message);
}

function natural(value) {
  return Number.isSafeInteger(value) && value >= 0;
}

function positive(value) {
  return Number.isSafeInteger(value) && value > 0;
}

export function assertCursorState(state) {
  if (
    !state ||
    !natural(state.processedThroughSeq) ||
    !positive(state.contextStartSeq) ||
    !natural(state.prunedThroughSeq) ||
    !positive(state.nextSeq)
  ) {
    reject(
      "cursor_shape",
      "Cursors must be safe integers and nextSeq/contextStartSeq must be positive",
    );
  }
  const {
    processedThroughSeq: processed,
    contextStartSeq: context,
    prunedThroughSeq: pruned,
    nextSeq,
  } = state;
  if (
    processed >= nextSeq ||
    context > nextSeq ||
    pruned > processed ||
    pruned >= context
  ) {
    reject(
      "cursor_order",
      "Require 0 <= pruned <= processed < nextSeq and pruned < context <= nextSeq",
    );
  }
  return true;
}

function boundaries(request, end) {
  const ends = request.completeTurnEnds;
  if (
    !Array.isArray(ends) ||
    !ends.every(positive) ||
    ends.some((n, i) => i > 0 && n <= ends[i - 1]) ||
    !ends.includes(end)
  ) {
    reject(
      "incomplete_interaction",
      "Target must be an explicitly attested complete-turn boundary",
    );
  }
}

function requireFence(request, through) {
  if (request.protectedFromSeq === undefined) return;
  if (!positive(request.protectedFromSeq)) {
    reject(
      "fence_shape",
      "protectedFromSeq must be a positive committed message sequence",
    );
  }
  if (through >= request.protectedFromSeq) {
    reject(
      "protected_interaction",
      "Transition would cross an unfinished interaction",
    );
  }
}

function requireAdvancement(current, key, next) {
  if (!natural(next) || next < current[key]) {
    reject("cursor_regression", "Cursor advancement must be monotone");
  }
  if (next === current[key]) {
    reject(
      "cursor_noop",
      "No-op does not create another transition or receipt",
    );
  }
}

function certifiedProcessingBatches(request, start, end) {
  if (!Array.isArray(request.batches) || request.batches.length === 0) {
    reject(
      "processing_unproven",
      "Every advanced complete turn needs a processing disposition",
    );
  }
  // A single disposition cannot silently attest several separately completed turns.
  // This specification oracle trusts the supplied boundary list; production must
  // derive it from persisted, owner-bound message/interaction records instead.
  const turns = request.completeTurnEnds.filter(
    (seq) => seq > start && seq <= end,
  );
  if (
    turns.length !== request.batches.length ||
    turns.some((seq, index) => request.batches[index]?.endSeq !== seq)
  ) {
    reject(
      "processing_gap",
      "Each completed turn must have its own ordered disposition",
    );
  }
  let previous = start;
  for (const batch of request.batches) {
    if (
      !positive(batch?.endSeq) ||
      batch.endSeq <= previous ||
      batch.endSeq > end
    ) {
      reject(
        "processing_gap",
        "Processing batches must be ordered, contiguous complete turns",
      );
    }
    boundaries(request, batch.endSeq);
    if (
      batch.disposition !== "durable_source" &&
      batch.disposition !== "no_long_term_value"
    ) {
      reject(
        "processing_disposition",
        "Every processed turn needs an explicit disposition",
      );
    }
    if (
      batch.disposition === "durable_source" &&
      (typeof batch.domainReceipt !== "string" || !batch.domainReceipt.trim())
    ) {
      reject(
        "missing_domain_receipt",
        "Durable source disposition requires the domain commit receipt",
      );
    }
    previous = batch.endSeq;
  }
  if (previous !== end)
    reject("processing_gap", "Processing attestations must reach the target");
}

function verifiedControlledCopies(request, through) {
  const proof = request.cleanupProof;
  if (
    !proof ||
    proof.inventoryComplete !== true ||
    proof.writesFenced !== true ||
    proof.sourceRefsRelinked !== true ||
    proof.durableEvidenceVerified !== true ||
    proof.clearedThroughSeq !== through
  ) {
    reject(
      "cleanup_unproven",
      "Inventory, source transfer, write fence and target cleanup must be proven",
    );
  }
  const required = proof.requiredSurfaces;
  const verified = proof.verifiedSurfaces;
  if (
    !Array.isArray(required) ||
    required.length === 0 ||
    !required.includes("messages") ||
    new Set(required).size !== required.length ||
    !Array.isArray(verified) ||
    new Set(verified.map((v) => v?.surface)).size !== verified.length
  ) {
    reject(
      "copy_inventory",
      "Declare a unique nonempty controlled-copy inventory including messages",
    );
  }
  const receipts = new Map(verified.map((v) => [v?.surface, v]));
  if (
    verified.length !== required.length ||
    required.some((surface) => {
      const item = receipts.get(surface);
      return (
        !item ||
        item.verified !== true ||
        !positive(item.clearedThroughSeq) ||
        item.clearedThroughSeq < through ||
        typeof item.receipt !== "string" ||
        !item.receipt.trim()
      );
    })
  ) {
    reject(
      "copy_not_verified",
      "Every controlled surface requires verified cleanup through target and a receipt",
    );
  }
}

export function advanceRetentionCursor(current, request) {
  assertCursorState(current);
  if (!request || typeof request !== "object")
    reject("transition_shape", "Transition required");

  const next = { ...current };
  const target = request.to;
  if (!natural(target))
    reject(
      "transition_shape",
      "Transition target must be a nonnegative safe integer",
    );

  if (request.kind === "process") {
    requireAdvancement(current, "processedThroughSeq", target);
    if (target >= current.nextSeq)
      reject("uncommitted_message", "Cannot process beyond committed messages");
    requireFence(request, target);
    boundaries(request, target);
    certifiedProcessingBatches(request, current.processedThroughSeq, target);
    next.processedThroughSeq = target;
  } else if (request.kind === "context") {
    requireAdvancement(current, "contextStartSeq", target);
    if (target > current.nextSeq)
      reject(
        "uncommitted_message",
        "Cannot move context beyond current message tail",
      );
    // First retained message is exactly after an entire completed turn.
    boundaries(request, target - 1);
    requireFence(request, target - 1);
    next.contextStartSeq = target;
  } else if (request.kind === "prune") {
    requireAdvancement(current, "prunedThroughSeq", target);
    // Unknown is not absence: explicit negative checks are mandatory before prune.
    if (
      [
        "activeRun",
        "pendingInteraction",
        "unsettledProposal",
        "liveCheckpoint",
      ].some((dependency) => request[dependency] !== false)
    ) {
      reject(
        "active_dependency",
        "Physical prune requires proof that every runtime dependency is closed",
      );
    }
    if (
      target > current.processedThroughSeq ||
      target >= current.contextStartSeq
    ) {
      reject(
        "unsafe_prune_boundary",
        "Prune must not pass processed or retained context",
      );
    }
    requireFence(request, target);
    boundaries(request, target);
    verifiedControlledCopies(request, target);
    next.prunedThroughSeq = target;
  } else {
    reject(
      "unknown_transition",
      "Only process, context and prune transitions are modeled",
    );
  }
  assertCursorState(next);
  return next;
}
