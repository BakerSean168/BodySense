import { useMemo, useState } from "react";
import { Image as ImageIcon, Play } from "lucide-react";
import { formatFileSize } from "../model/profileCategories";
import {
  groupMediaByMonth,
  limitMediaGroups,
  type PersonMediaTile,
  type PersonMediaUpload,
} from "../model/personMedia";

const STATE_LABELS: Record<PersonMediaTile["state"], string | null> = {
  ready: null,
  processing: "处理中…",
  failed: "处理失败",
  source_unavailable: "原件已移除",
};

/**
 * Penpot "Desktop / Mobile · Person / Media": month groups of thumbnail tiles
 * (4 columns desktop, 2 mobile) with name and "size · date" beneath, and
 * "查看全部 N 个" once more than the first rows exist. Tiles are not controls:
 * there is no media detail view, so they stay plain list items.
 */
export function PersonMediaGrid({
  uploads,
  initialLimit,
  now,
}: {
  uploads: readonly PersonMediaUpload[];
  /** Tiles shown before "查看全部" (Penpot: 8 desktop, 6 mobile). */
  initialLimit: number;
  now?: Date;
}) {
  const [showAll, setShowAll] = useState(false);
  const groups = useMemo(() => groupMediaByMonth(uploads, now), [uploads, now]);
  const visible = showAll ? groups : limitMediaGroups(groups, initialLimit);
  const hidden = uploads.length > initialLimit && !showAll;

  return (
    <div className="bc-person-media" id="bc-person-media-groups">
      {visible.map((group) => (
        <section
          key={group.key}
          className="bc-person-media-group"
          aria-labelledby={`bc-media-group-${group.key}`}
        >
          <h3
            id={`bc-media-group-${group.key}`}
            className="bc-person-media-month"
          >
            {group.label}
          </h3>
          <ul className="bc-person-media-grid">
            {group.tiles.map((tile) => (
              <MediaTile key={tile.upload.id} tile={tile} />
            ))}
          </ul>
        </section>
      ))}
      {hidden ? (
        <button
          type="button"
          className="bc-text-button bc-person-media-more"
          aria-expanded="false"
          aria-controls="bc-person-media-groups"
          onClick={() => setShowAll(true)}
        >
          查看全部 {uploads.length} 个
        </button>
      ) : null}
    </div>
  );
}

function MediaTile({ tile }: { tile: PersonMediaTile }) {
  const { upload, kind, state, dateLabel } = tile;
  const overlay = STATE_LABELS[state];
  return (
    <li
      className="bc-person-media-tile"
      data-kind={kind}
      data-state={state}
      data-testid="person-media-tile"
    >
      <div className="bc-person-media-thumb">
        <span className="bc-person-media-glyph" aria-hidden="true">
          {kind === "video" ? <Play size={18} /> : <ImageIcon size={26} />}
        </span>
        {kind === "video" ? (
          <span className="bc-person-media-badge">视频</span>
        ) : null}
        {overlay ? (
          <span className="bc-person-media-overlay">{overlay}</span>
        ) : null}
      </div>
      <strong className="bc-person-media-name" title={upload.original_name}>
        {upload.original_name}
      </strong>
      <span className="bc-person-media-meta">
        {formatFileSize(upload.file_size)} · {dateLabel}
      </span>
    </li>
  );
}
