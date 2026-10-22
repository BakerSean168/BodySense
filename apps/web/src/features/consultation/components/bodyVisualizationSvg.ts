export interface ViewSVGProps {
  highlightedIds: Set<string>;
  onPartClick: (partName: string) => void;
}

export function getFill(id: string, highlightedIds: Set<string>): string {
  return highlightedIds.has(id) ? "url(#pain-gradient)" : "url(#clay-gradient)";
}

export function getStroke(id: string, highlightedIds: Set<string>): string {
  return highlightedIds.has(id) ? "#B65E49" : "#C4BFAF";
}

export function getFilter(
  id: string,
  highlightedIds: Set<string>,
): string | undefined {
  return highlightedIds.has(id) ? "url(#glow-filter)" : undefined;
}
