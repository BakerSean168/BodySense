import { useEffect, useState } from "react";
import { useMediaQuery } from "@/hooks/useMediaQuery";

export type ThemePreference = "light" | "dark" | "system";
function savedTheme(): ThemePreference {
  try {
    const value = localStorage.getItem("bodysense.canvas.theme");
    return value === "dark" || value === "system" ? value : "light";
  } catch {
    return "light";
  }
}

/** Persisted canvas appearance is unrelated to window or scene state. */
export function useCanvasAppearance() {
  const [themePreference, setThemePreference] =
    useState<ThemePreference>(savedTheme);
  const systemDark = useMediaQuery("(prefers-color-scheme: dark)");
  const dark =
    themePreference === "dark" || (themePreference === "system" && systemDark);
  useEffect(() => {
    try {
      localStorage.setItem("bodysense.canvas.theme", themePreference);
    } catch {
      /* Preferences never gate the workspace. */
    }
  }, [themePreference]);

  return { themePreference, setThemePreference, dark };
}
