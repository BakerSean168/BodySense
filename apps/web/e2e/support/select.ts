import type { Locator } from "@playwright/test";

/**
 * Picks an option in a components/ui Select (Base UI): opens the trigger and
 * clicks the item whose `data-value` matches. Replaces `locator.selectOption`
 * now that the app no longer renders native `<select>` elements.
 */
export async function selectOption(trigger: Locator, value: string) {
  await trigger.click();
  await trigger
    .page()
    .locator(
      `[data-slot="select-content"] [data-slot="select-item"][data-value=${JSON.stringify(value)}]`,
    )
    .click();
}
