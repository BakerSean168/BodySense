import {
  useEffect,
  useId,
  useRef,
  useState,
  type ComponentType,
  type KeyboardEvent,
} from "react";
import {
  ChevronDown,
  FileText,
  Image as ImageIcon,
  List,
  Repeat,
} from "lucide-react";
import { DockTrainingIcon } from "../../components/DockIcons";
import {
  PROFILE_CATEGORIES,
  getProfileCategoryMeta,
  type ProfileCategoryId,
} from "../../model/profileCategories";

// Penpot menu glyphs: equipment is the level barbell the Dock uses for 训练,
// habits the two-arrow repeat loop.
const ICONS: Record<
  ProfileCategoryId,
  ComponentType<{ size?: number; "aria-hidden"?: boolean | "true" }>
> = {
  records: List,
  equipment: DockTrainingIcon,
  media: ImageIcon,
  reports: FileText,
  habits: Repeat,
};

/**
 * Penpot "Overlay / Person / Category menu": the header picker opens a small
 * "切换分类" card with an icon, the category name and its count. Built as an
 * ARIA listbox (WAI-ARIA APG "select-only combobox" interaction): the trigger
 * opens it with Enter / Space / ↓ / ↑; ↑ ↓ Home End move the active option;
 * Enter / Space choose; Escape and Tab close; focus returns to the trigger.
 * Counts appear only for categories whose data was already read; nothing is
 * estimated.
 */
export function PersonCategoryMenu({
  value,
  onChange,
  counts = {},
}: {
  value: ProfileCategoryId;
  onChange: (category: ProfileCategoryId) => void;
  counts?: Partial<Record<ProfileCategoryId, string>>;
}) {
  const id = useId();
  const labelId = `${id}-label`;
  const valueId = `${id}-value`;
  const listId = `${id}-list`;
  const optionId = (category: ProfileCategoryId) => `${id}-opt-${category}`;
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState<ProfileCategoryId>(value);
  const trigger = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLUListElement>(null);
  const root = useRef<HTMLSpanElement>(null);
  const meta = getProfileCategoryMeta(value);

  useEffect(() => {
    if (open) list.current?.focus();
  }, [open]);

  // An outside pointer press closes the menu without changing the category.
  useEffect(() => {
    if (!open) return;
    const onPointer = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", onPointer);
    return () => document.removeEventListener("pointerdown", onPointer);
  }, [open]);

  const show = (start: ProfileCategoryId = value) => {
    setActive(start);
    setOpen(true);
  };
  const close = (refocus = true) => {
    setOpen(false);
    if (refocus) trigger.current?.focus();
  };
  const choose = (category: ProfileCategoryId) => {
    close();
    if (category !== value) onChange(category);
  };
  const ids = PROFILE_CATEGORIES.map((category) => category.id);
  const move = (delta: number) => {
    const index = ids.indexOf(active);
    setActive(ids[Math.min(ids.length - 1, Math.max(0, index + delta))]);
  };

  const onTriggerKey = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      show();
    }
  };
  const onListKey = (event: KeyboardEvent<HTMLUListElement>) => {
    switch (event.key) {
      case "ArrowDown":
        move(1);
        break;
      case "ArrowUp":
        move(-1);
        break;
      case "Home":
        setActive(ids[0]);
        break;
      case "End":
        setActive(ids[ids.length - 1]);
        break;
      case "Enter":
      case " ":
        choose(active);
        break;
      case "Escape":
        close();
        break;
      case "Tab":
        close(false);
        return;
      default:
        return;
    }
    // Keep Escape and arrows from reaching the window / canvas shortcuts.
    event.preventDefault();
    event.stopPropagation();
  };

  return (
    <span className="bc-category-menu" ref={root}>
      <span id={labelId} className="sr-only">
        档案分类
      </span>
      <button
        ref={trigger}
        type="button"
        className="bc-window-context-select bc-category-trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        aria-labelledby={`${labelId} ${valueId}`}
        data-value={value}
        onClick={() => (open ? close() : show())}
        onKeyDown={onTriggerKey}
      >
        <span id={valueId}>{meta.label}</span>
        <ChevronDown size={15} aria-hidden="true" />
      </button>
      {open ? (
        <div className="bc-category-popover">
          <p className="bc-category-popover-title" aria-hidden="true">
            切换分类
          </p>
          <ul
            ref={list}
            id={listId}
            role="listbox"
            tabIndex={-1}
            aria-labelledby={labelId}
            aria-activedescendant={optionId(active)}
            onKeyDown={onListKey}
            onBlur={(event) => {
              if (!root.current?.contains(event.relatedTarget)) setOpen(false);
            }}
          >
            {PROFILE_CATEGORIES.map((category) => {
              const Icon = ICONS[category.id];
              const count = counts[category.id];
              return (
                <li
                  key={category.id}
                  id={optionId(category.id)}
                  role="option"
                  aria-selected={category.id === value}
                  data-active={category.id === active ? "true" : undefined}
                  className="bc-category-option"
                  onPointerMove={() => setActive(category.id)}
                  onClick={() => choose(category.id)}
                >
                  <Icon size={20} aria-hidden="true" />
                  <span className="bc-category-option-label">
                    {category.label}
                  </span>
                  {count ? (
                    <span className="bc-category-option-count">{count}</span>
                  ) : null}
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}
    </span>
  );
}
