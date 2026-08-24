import type { StreamEvent } from "../types/consultation";
import {
  dispatchReplayEvents,
  type SSEHandlers,
  type SSEParseState,
} from "../hooks/useSSEProcessor";

export interface DurableRunEventPage {
  events: StreamEvent[];
  hasMore: boolean;
  nextAfterSeq: number | null;
}

interface RecoverDurableRunOptions {
  fetchPage: (afterSeq: number) => Promise<DurableRunEventPage>;
  handlers: SSEHandlers;
  afterSeq?: number;
  timeoutMs?: number;
  pollIntervalMs?: number;
  now?: () => number;
  sleep?: (ms: number) => Promise<void>;
  signal?: AbortSignal;
}

export interface DurableRunRecoveryResult {
  maxSeq: number;
  terminalType:
    | "stream.done"
    | "stream.error"
    | "run.failed"
    | "run.cancelled";
}

const TERMINAL_TYPES = new Set<StreamEvent["type"]>([
  "stream.done",
  "stream.error",
  "run.failed",
  "run.cancelled",
]);

const defaultSleep = (ms: number) =>
  new Promise<void>((resolve) => globalThis.setTimeout(resolve, ms));

/**
 * Poll the durable Runtime Event Log after a POST/SSE transport disconnect.
 *
 * The server continues the Agent run independently of the HTTP request, so an
 * empty page means "not committed yet", not "the run ended". Recovery only
 * finishes after an explicit persisted stream.done/stream.error terminal event.
 */
export async function recoverDurableRunEvents({
  fetchPage,
  handlers,
  afterSeq = 0,
  timeoutMs = 5 * 60_000,
  pollIntervalMs = 300,
  now = Date.now,
  sleep = defaultSleep,
  signal,
}: RecoverDurableRunOptions): Promise<DurableRunRecoveryResult> {
  const deadline = now() + timeoutMs;
  let state: SSEParseState = { currentEvent: "", maxSeq: afterSeq };

  let lastFetchError: Error | null = null;

  while (now() < deadline) {
    if (signal?.aborted) {
      throw new Error("durable run recovery cancelled");
    }
    let page: DurableRunEventPage;
    try {
      page = await fetchPage(state.maxSeq);
      lastFetchError = null;
    } catch (error) {
      lastFetchError =
        error instanceof Error ? error : new Error(String(error));
      await sleep(pollIntervalMs);
      if (signal?.aborted) {
        throw new Error("durable run recovery cancelled", { cause: error });
      }
      continue;
    }
    const freshEvents = [...page.events]
      .filter(
        (event) => typeof event.seq !== "number" || event.seq > state.maxSeq,
      )
      .sort((left, right) => (left.seq ?? 0) - (right.seq ?? 0));

    state = dispatchReplayEvents(freshEvents, handlers, state);

    const terminal = freshEvents.find((event) =>
      TERMINAL_TYPES.has(event.type),
    );
    if (
      terminal?.type === "stream.done" ||
      terminal?.type === "stream.error" ||
      terminal?.type === "run.failed" ||
      terminal?.type === "run.cancelled"
    ) {
      return { maxSeq: state.maxSeq, terminalType: terminal.type };
    }

    if (page.hasMore) {
      continue;
    }
    await sleep(pollIntervalMs);
    if (signal?.aborted) {
      throw new Error("durable run recovery cancelled");
    }
  }

  const detail = lastFetchError?.message
    ? `：${lastFetchError.message}`
    : "";
  throw new Error(
    `恢复本次执行超时。服务可能仍在恢复中，请稍后继续输入或重新加载${detail}`,
  );
}
