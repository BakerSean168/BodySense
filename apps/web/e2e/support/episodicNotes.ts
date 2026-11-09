import { execFile } from "node:child_process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

export interface EpisodicNoteSeed {
  contentText: string;
  fromSeq: number;
  toSeq: number;
}

/**
 * S6-B exposes inspection and deletion only: note generation is still an
 * unwired S2 boundary. The 390px browser check therefore seeds inert shadow
 * rows directly into the disposable E2E PostgreSQL (never a hosted database,
 * never a real account). The SQL mirrors the S2 storage shape and creates no
 * clinical data. E2E_DATABASE_URL is required so a missing database fails
 * loudly instead of silently skipping the check.
 */
export async function seedEpisodicNotes(
  userId: string,
  conversationId: string,
  notes: readonly EpisodicNoteSeed[],
): Promise<void> {
  const databaseURL = process.env.E2E_DATABASE_URL;
  if (!databaseURL) {
    throw new Error(
      "E2E_DATABASE_URL is required to seed S6-B episodic notes; refusing to skip the browser check",
    );
  }
  const quote = (value: string) => `'${value.replace(/'/g, "''")}'`;
  const rows = notes.map((note) =>
    [
      quote(crypto.randomUUID()),
      `${quote(userId)}::uuid`,
      `${quote(conversationId)}::uuid`,
      String(note.fromSeq),
      String(note.toSeq),
      `ARRAY[${quote(crypto.randomUUID())}]::uuid[]`,
      quote("e2e-episodic-generator-v1"),
      quote("e2e-episodic-retention-v1"),
      quote(note.contentText),
      quote("shadow"),
      "now() - interval '1 minute'",
      "now() - interval '1 minute'",
    ].join(","),
  );
  const sql = `INSERT INTO episodic_notes (
    id, user_id, conversation_id, from_seq, to_seq, source_message_ids,
    generator_version, retention_policy_version, content_text, status,
    created_at, updated_at
  ) VALUES ${rows.map((row) => `(${row})`).join(",\n")}`;
  await execFileAsync(
    "psql",
    [databaseURL, "-X", "-v", "ON_ERROR_STOP=1", "-c", sql],
    { env: process.env },
  );
}
