import { useEffect, useId, useMemo, useRef, useState } from "react";
import { Check, Search } from "lucide-react";
import {
  BODY_REGION_IDS,
  getBodyRegionDefinition,
  normalizeBodyRegionInput,
  type BodyRegionId,
} from "@/features/body-explorer/model/bodyRegionOntology";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import {
  regionPaletteShortcut,
  type CanvasSelection,
} from "../model/bodyCanvas";

/** ⌘K command palette: choose a body region by name, id or English label. */
export function CanvasRegionPicker({
  visible,
  selection,
  themeClass,
  onSelect,
  onVisibleChange,
  onCardOpenChange,
}: {
  visible: boolean;
  selection: CanvasSelection;
  themeClass: string;
  onSelect: (regions: BodyRegionId[], append?: boolean) => void;
  onVisibleChange: (visible: boolean) => void;
  onCardOpenChange: (visible: boolean) => void;
}) {
  const listId = useId();
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  const results = useMemo(() => {
    const normalized = normalizeBodyRegionInput(query);
    return BODY_REGION_IDS.map(getBodyRegionDefinition).filter((region) => {
      if (!normalized) return true;
      return [
        region.id,
        region.labels["zh-CN"],
        region.labels.en,
        ...region.aliases,
      ].some((candidate) =>
        normalizeBodyRegionInput(candidate).includes(normalized),
      );
    });
  }, [query]);

  const active = results[Math.min(activeIndex, results.length - 1)] ?? null;
  const optionId = (id: BodyRegionId) => `${listId}-${id}`;

  useEffect(() => {
    if (!visible) return;
    setQuery("");
    setActiveIndex(0);
  }, [visible]);

  useEffect(() => {
    if (!active) return;
    listRef.current
      ?.querySelector<HTMLElement>(`[data-region-id="${active.id}"]`)
      ?.scrollIntoView?.({ block: "nearest" });
  }, [active]);

  const choose = (id: BodyRegionId, append: boolean) => {
    onSelect([id], append);
    if (append) {
      setQuery("");
      setActiveIndex(0);
      inputRef.current?.focus();
      return;
    }
    onVisibleChange(false);
    onCardOpenChange(true);
  };

  return (
    <Dialog
      open={visible}
      onOpenChange={(open) => {
        if (!open) onVisibleChange(false);
      }}
    >
      <DialogContent
        showCloseButton={false}
        className={`${themeClass} bc-region-palette`}
      >
        <DialogTitle className="sr-only">按名称选择身体区域</DialogTitle>
        <div className="bc-region-palette-search">
          <Search size={18} strokeWidth={1.8} aria-hidden="true" />
          <input
            ref={inputRef}
            role="combobox"
            aria-label="搜索身体区域"
            aria-autocomplete="list"
            aria-expanded={results.length > 0}
            aria-controls={results.length > 0 ? listId : undefined}
            aria-activedescendant={active ? optionId(active.id) : undefined}
            placeholder="搜索身体区域"
            value={query}
            autoFocus
            onChange={(event) => {
              setQuery(event.target.value);
              setActiveIndex(0);
            }}
            onKeyDown={(event) => {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                if (results.length)
                  setActiveIndex(
                    (index) =>
                      (Math.min(index, results.length - 1) + 1) %
                      results.length,
                  );
              } else if (event.key === "ArrowUp") {
                event.preventDefault();
                if (results.length)
                  setActiveIndex(
                    (index) =>
                      (Math.min(index, results.length - 1) -
                        1 +
                        results.length) %
                      results.length,
                  );
              } else if (event.key === "Enter" && active) {
                event.preventDefault();
                choose(active.id, event.shiftKey);
              }
            }}
          />
          <kbd aria-hidden="true">{regionPaletteShortcut()}</kbd>
        </div>
        {results.length > 0 ? (
          <ul
            ref={listRef}
            id={listId}
            role="listbox"
            aria-label="身体区域"
            className="bc-region-palette-list"
          >
            {results.map((region, index) => {
              const selected = selection.regions.includes(region.id);
              return (
                <li
                  key={region.id}
                  id={optionId(region.id)}
                  role="option"
                  aria-selected={selected}
                  data-active={index === activeIndex ? "true" : undefined}
                  data-region-id={region.id}
                  className="bc-region-palette-option"
                  onPointerMove={() => setActiveIndex(index)}
                  onClick={(event) =>
                    choose(
                      region.id,
                      event.shiftKey ||
                        event.metaKey ||
                        event.ctrlKey ||
                        event.altKey,
                    )
                  }
                >
                  <span>{region.labels["zh-CN"]}</span>
                  {selected ? (
                    <Check size={16} strokeWidth={2} aria-hidden="true" />
                  ) : null}
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="bc-region-palette-empty">没有匹配的区域</p>
        )}
      </DialogContent>
    </Dialog>
  );
}
