import { expect, type APIRequestContext } from "@playwright/test";
import { parseStreamEvent, type StreamEvent } from "@bodysense/contracts";

const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

/**
 * Read a Run's events from the durable Run event log.
 *
 * Specs must not read a chat stream through `page.waitForResponse(...).text()`.
 * The client races its live SSE reader against a durable watcher and cancels
 * the live reader as soon as the watcher has seen the terminal event; Chromium
 * then drops the cancelled response body ("No data found for resource"), so
 * the read fails whenever the watcher wins, typically on a slow or cold first
 * page. The durable log is the authority the client itself and every second
 * device recover from, and it is complete once `until` holds.
 */
export async function readRunEvents(
  request: APIRequestContext,
  headers: Record<string, string>,
  conversationId: string,
  runId: string,
  {
    afterSeq = 0,
    until,
  }: { afterSeq?: number; until: (events: StreamEvent[]) => boolean },
): Promise<StreamEvent[]> {
  let events: StreamEvent[] = [];
  await expect
    .poll(
      async () => {
        const response = await request.get(
          `${api}/api/v1/conversations/${conversationId}/runs/${runId}/events?after_seq=${afterSeq}&limit=1000`,
          { headers },
        );
        expect(response.ok(), await response.text()).toBeTruthy();
        const page = (await response.json()) as {
          events: Array<Omit<StreamEvent, "version">>;
        };
        events = page.events.map((item) =>
          parseStreamEvent({
            version: 1,
            seq: item.seq,
            channel: item.channel,
            type: item.type,
            ids: item.ids,
            payload: item.payload,
          }),
        );
        return until(events);
      },
      { timeout: 30_000 },
    )
    .toBe(true);
  return events;
}

/** The conversation a chat page has navigated to after its first message. */
export function conversationIdFromUrl(url: string): string {
  const match = /\/consultation\/([0-9a-f-]{36})(?:[/?#]|$)/.exec(url);
  if (!match) throw new Error(`No conversation in ${url}`);
  return match[1];
}

export const isTerminal = (event: StreamEvent) =>
  event.type === "stream.done" || event.type === "stream.error";
