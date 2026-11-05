// The local calendar date (YYYY-MM-DD) of a date-picker value or Date.
// Never use toISOString() for this: it converts to UTC first, so in the
// Philippines/Singapore (UTC+8) a date picked at local midnight becomes
// the previous day.
export function localDate(v: number | string | Date = new Date()): string {
  const d = v instanceof Date ? v : new Date(v);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}
