import { cva, type VariantProps } from "class-variance-authority";

/**
 * Shared look for the app's form fields (Input, Textarea, Select and
 * DatePicker triggers). Colours come from the `--ui-*` tokens in index.css,
 * which follow the body-canvas paper theme (and its dark mode) and fall back
 * to the light paper palette elsewhere.
 *
 * - `field`: rounded, hairline-bordered box (records, region picker, drawers).
 * - `underline`: label-less underline, matching the identity and auth fields.
 * - `bare`: no frame at all, for rows that already draw their own underline.
 * - `ghost`: quiet header picker (window context selects).
 */
export const fieldVariants = cva(
  "w-full min-w-0 text-(--ui-ink) outline-none transition-[color,border-color,box-shadow,background-color] placeholder:text-(--ui-muted) disabled:cursor-not-allowed disabled:opacity-55 data-disabled:cursor-not-allowed data-disabled:opacity-55",
  {
    variants: {
      variant: {
        field:
          "min-h-10 rounded-[10px] border border-(--ui-line) bg-(--ui-field) px-3 text-sm font-normal hover:border-[color-mix(in_srgb,var(--ui-primary)_35%,var(--ui-line))] focus-visible:border-(--ui-primary) focus-visible:ring-3 focus-visible:ring-[color-mix(in_srgb,var(--ui-primary)_16%,transparent)] aria-invalid:border-(--ui-danger) data-popup-open:border-(--ui-primary)",
        underline:
          "min-h-9 rounded-none border-0 border-b border-(--ui-line) bg-transparent px-0 text-sm font-semibold focus-visible:border-(--ui-primary) focus-visible:shadow-[0_1px_0_var(--ui-primary)] aria-invalid:border-(--ui-danger) data-popup-open:border-(--ui-primary)",
        bare: "min-h-8 rounded-none border-0 bg-transparent px-0 text-sm font-semibold",
        ghost:
          "min-h-[34px] rounded-lg border-0 bg-transparent px-2.5 text-xs font-semibold hover:bg-[color-mix(in_srgb,var(--ui-highlight)_60%,transparent)] focus-visible:ring-2 focus-visible:ring-(--ui-primary) data-popup-open:bg-[color-mix(in_srgb,var(--ui-highlight)_70%,transparent)]",
      },
    },
    defaultVariants: { variant: "field" },
  },
);

export type FieldVariant = NonNullable<
  VariantProps<typeof fieldVariants>["variant"]
>;

/** Rounded paper popup shared by Select, Popover and the date picker. */
export const popupSurface =
  "rounded-xl border border-(--ui-paper-line) bg-(--ui-paper) text-(--ui-ink) shadow-(--ui-popup-shadow) outline-none";
