/**
 * Profile preference semantics (PC-D1, DC-040).
 *
 * Preferences only change presentation and the interpretation of new input.
 * They never rewrite a stored Observation value/unit, a Trainer training date
 * or a historical occurrence time: every helper here formats a copy and keeps
 * the original value reachable. An unset preference falls back to a default
 * that is always labelled as a default, never presented as the user's choice.
 */

export type UnitPreference = "metric" | "imperial";

export interface ProfilePreferencesLike {
  locale?: string;
  time_zone?: string;
  unit_preference?: UnitPreference;
}

/** The browser's own zone; used only when the user has not chosen one. */
export function deviceTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

export interface EffectiveTimeZone {
  zone: string;
  /** True when the zone is the device default rather than a stored choice. */
  isDefault: boolean;
}

export function effectiveTimeZone(
  preferences: ProfilePreferencesLike | null | undefined,
): EffectiveTimeZone {
  const chosen = preferences?.time_zone;
  if (chosen && isSupportedTimeZone(chosen)) {
    return { zone: chosen, isDefault: false };
  }
  return { zone: deviceTimeZone(), isDefault: true };
}

export function effectiveUnitPreference(
  preferences: ProfilePreferencesLike | null | undefined,
): { unit: UnitPreference; isDefault: boolean } {
  const chosen = preferences?.unit_preference;
  return chosen
    ? { unit: chosen, isDefault: false }
    : { unit: "metric", isDefault: true };
}

export function isSupportedTimeZone(zone: string): boolean {
  if (!zone || zone === "Local") return false;
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: zone });
    return true;
  } catch {
    return false;
  }
}

const FALLBACK_TIME_ZONES = [
  "Asia/Shanghai",
  "Asia/Hong_Kong",
  "Asia/Taipei",
  "Asia/Tokyo",
  "Asia/Singapore",
  "Europe/London",
  "Europe/Berlin",
  "America/New_York",
  "America/Los_Angeles",
  "Australia/Sydney",
  "UTC",
];

/** IANA zones the browser can format, with the given zones always included. */
export function selectableTimeZones(...include: Array<string | undefined>): string[] {
  let zones: string[] = FALLBACK_TIME_ZONES;
  const intl = Intl as typeof Intl & {
    supportedValuesOf?: (key: "timeZone") => string[];
  };
  if (typeof intl.supportedValuesOf === "function") {
    try {
      zones = intl.supportedValuesOf("timeZone");
    } catch {
      zones = FALLBACK_TIME_ZONES;
    }
  }
  const set = new Set(zones);
  for (const zone of include) {
    if (zone && isSupportedTimeZone(zone)) set.add(zone);
  }
  return [...set].sort((a, b) => a.localeCompare(b));
}

/** Formats an instant on the preferred clock; the instant itself is unchanged. */
export function formatInstantInZone(
  iso: string | null | undefined,
  zone: string,
  options: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  },
): string | null {
  if (!iso) return null;
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return null;
  return new Intl.DateTimeFormat("zh-CN", { ...options, timeZone: zone }).format(
    date,
  );
}

export interface MeasurementDisplay {
  /** What the user sees in their preferred system. */
  text: string;
  /** The stored value exactly as recorded, when the display converted it. */
  original: string | null;
}

function trimNumber(value: number, digits: number): string {
  return Number(value.toFixed(digits)).toString();
}

/**
 * Converts length (cm, m) and mass (kg) for display only. Any other unit is
 * shown as recorded: the preference never invents a conversion.
 */
export function formatMeasurement(
  value: number,
  unit: string,
  preference: UnitPreference,
): MeasurementDisplay {
  const recorded = `${trimNumber(value, 2)} ${unit}`;
  if (preference === "metric" || !Number.isFinite(value)) {
    return { text: recorded, original: null };
  }
  const normalized = unit.trim().toLowerCase();
  if (normalized === "cm" || normalized === "m") {
    const totalInches = (normalized === "m" ? value * 100 : value) / 2.54;
    let feet = Math.floor(totalInches / 12);
    let inches = Math.round(totalInches - feet * 12);
    if (inches === 12) {
      feet += 1;
      inches = 0;
    }
    return { text: `${feet} ft ${inches} in`, original: recorded };
  }
  if (normalized === "kg") {
    return { text: `${trimNumber(value * 2.20462262, 1)} lb`, original: recorded };
  }
  return { text: recorded, original: null };
}
