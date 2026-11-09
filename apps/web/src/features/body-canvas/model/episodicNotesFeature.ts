/**
 * S6-B kill switch. The memory category is a preview surface that must stay
 * invisible until product and privacy owners approve its copy and retention.
 * It therefore reads a build-time Vite flag and defaults off.
 */
export function episodicNotesControlsEnabled(): boolean {
  const raw = import.meta.env.VITE_EPISODIC_NOTES_CONTROLS_ENABLED;
  return raw === "true" || raw === "1";
}
