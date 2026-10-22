import type { AnatomyStructureSummary } from "../components/AnatomyInspector";

export function buildBreadcrumb(
  selected: AnatomyStructureSummary | null,
  index: ReadonlyMap<string, AnatomyStructureSummary>,
): AnatomyStructureSummary[] {
  if (!selected) return [];
  const path: AnatomyStructureSummary[] = [];
  const seen = new Set<string>();
  let current: AnatomyStructureSummary | undefined = selected;
  while (current && !seen.has(current.id)) {
    seen.add(current.id);
    path.unshift(current);
    current = current.parentId ? index.get(current.parentId) : undefined;
  }
  return path;
}
