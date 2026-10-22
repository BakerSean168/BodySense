import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

type User = Pick<typeof userEvent, "click">;

/**
 * Picks an option in a components/ui Select (the replacement for
 * `userEvent.selectOptions`). `choice` is the option value (matched on
 * `data-value`) or its visible label.
 */
export async function selectOption(
  trigger: HTMLElement,
  choice: string,
  user: User = userEvent,
) {
  await user.click(trigger);
  const listbox = await screen.findByRole("listbox");
  const options = within(listbox).getAllByRole("option");
  const target =
    options.find((option) => option.getAttribute("data-value") === choice) ??
    options.find((option) => option.textContent?.trim() === choice);
  if (!target) {
    throw new Error(
      `No option "${choice}" in [${options.map((o) => o.getAttribute("data-value")).join(", ")}]`,
    );
  }
  await user.click(target);
}

/** Opens a Select and returns its options (closed Selects render none). */
export async function openOptions(
  trigger: HTMLElement,
  user: User = userEvent,
) {
  await user.click(trigger);
  const listbox = await screen.findByRole("listbox");
  return within(listbox).getAllByRole("option");
}
