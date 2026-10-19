/**
 * Penpot Chat header "对话 / 今天 ▾" and its history list, projected from the
 * user's single Conversation. Each day is one where the user actually wrote
 * something (created_at on the display clock); nothing is grouped by guess,
 * and interaction answers (shown as the ask card, not as a message) are left
 * out. Days run newest first.
 */
export interface ChatHistoryMessage {
  id: string;
  role: string;
  created_at: string;
  content_text?: string | null;
  parts?: ReadonlyArray<{ type: string; text?: string }>;
  metadata?: Record<string, unknown> | null;
}

export interface ChatHistoryDay {
  /** YYYY-MM-DD on the display clock. */
  day: string;
  /** 今天 / 昨天 / 10月14日 (with the year when it is not this year). */
  label: string;
  /** Messages the user wrote that day. */
  count: number;
  /** The first thing the user wrote that day, trimmed to one line. */
  preview: string;
  /** Where the thread scrolls to for this day. */
  firstMessageId: string;
}

export interface ChatHistory {
  todayKey: string;
  days: readonly ChatHistoryDay[];
}

function dayKey(instant: Date, zone: string | undefined): string | null {
  if (Number.isNaN(instant.getTime())) return null;
  try {
    // en-CA formats as YYYY-MM-DD.
    return new Intl.DateTimeFormat("en-CA", {
      timeZone: zone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(instant);
  } catch {
    return null;
  }
}

function previousDay(key: string): string {
  const [y, m, d] = key.split("-").map(Number);
  const date = new Date(Date.UTC(y!, m! - 1, d! - 1));
  return date.toISOString().slice(0, 10);
}

export function chatDayLabel(key: string, todayKey: string): string {
  if (key === todayKey) return "今天";
  if (key === previousDay(todayKey)) return "昨天";
  const [y, m, d] = key.split("-").map(Number);
  const sameYear = key.slice(0, 4) === todayKey.slice(0, 4);
  return sameYear ? `${m}月${d}日` : `${y}年${m}月${d}日`;
}

function messageText(message: ChatHistoryMessage): string {
  const text =
    message.content_text ||
    (message.parts ?? [])
      .filter((part) => part.type === "text" && part.text)
      .map((part) => part.text)
      .join("");
  return text.replace(/\s+/g, " ").trim();
}

export function groupChatHistory(
  messages: readonly ChatHistoryMessage[],
  zone: string | undefined,
  now: Date = new Date(),
): ChatHistory {
  const todayKey = dayKey(now, zone) ?? now.toISOString().slice(0, 10);
  const byDay = new Map<string, ChatHistoryDay>();
  const ordered = [...messages].sort(
    (a, b) => Date.parse(a.created_at) - Date.parse(b.created_at),
  );
  for (const message of ordered) {
    if (message.role !== "user") continue;
    if (message.metadata?.is_interaction_answer === true) continue;
    const text = messageText(message);
    if (!text) continue;
    const key = dayKey(new Date(message.created_at), zone);
    if (!key) continue;
    const entry = byDay.get(key);
    if (entry) entry.count += 1;
    else
      byDay.set(key, {
        day: key,
        label: chatDayLabel(key, todayKey),
        count: 1,
        preview: text.length > 40 ? `${text.slice(0, 40)}…` : text,
        firstMessageId: message.id,
      });
  }
  const days = [...byDay.values()].sort((a, b) => b.day.localeCompare(a.day));
  return { todayKey, days };
}
