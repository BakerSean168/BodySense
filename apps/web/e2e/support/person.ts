import { expect, type Page } from "@playwright/test";

const LABELS = {
  records: "身体记录",
  equipment: "器材",
  media: "媒体",
  reports: "报告",
  habits: "生活习惯",
} as const;

export type PersonCategory = keyof typeof LABELS;

/** The Person header trigger for the Penpot category menu (an ARIA listbox). */
export function personCategoryTrigger(page: Page) {
  return page.getByRole("button", { name: /^档案分类/ });
}

/** Opens the Penpot category menu and chooses a category by its option. */
export async function choosePersonCategory(page: Page, category: PersonCategory) {
  const trigger = personCategoryTrigger(page);
  await trigger.click();
  const list = page.getByRole("listbox", { name: "档案分类" });
  await expect(list).toBeVisible();
  await list
    .getByRole("option", { name: new RegExp(`^${LABELS[category]}`) })
    .click();
  await expect(trigger).toHaveAttribute("data-value", category);
}
