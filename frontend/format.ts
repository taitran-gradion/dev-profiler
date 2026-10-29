import { formatNumber, type useLocale } from "@margince/frontend/app";

type Locale = ReturnType<typeof useLocale>["locale"];

/** What a figure the answer did not carry prints as. */
export const MISSING = "-";

export function ms(value: number | undefined, locale: Locale): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return MISSING;
  return `${formatNumber(Math.round(value * 10) / 10, locale)} ms`;
}

export function count(value: number | undefined, locale: Locale): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return MISSING;
  return formatNumber(value, locale);
}

export function statusTone(status: number) {
  if (status >= 500) return "danger";
  if (status >= 400) return "warning";
  return "success";
}

/** A span's detail as one line of `key=value` pairs. */
export function detailLine(detail?: Record<string, string>): string {
  return Object.entries(detail ?? {})
    .map(([k, v]) => `${k}=${v}`)
    .join(" ");
}
