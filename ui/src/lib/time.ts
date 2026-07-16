/*
SQLite hands out timestamps like "2026-07-15 04:13:37" in UTC without a
zone marker; normalize before parsing so relative times don't drift by
the viewer's UTC offset.
*/
export function parseTimestamp(raw: string): Date | null {
  if (!raw) return null;
  let s = raw.trim().replace(' ', 'T');
  if (!/Z|[+-]\d{2}:?\d{2}$/.test(s)) s += 'Z';
  const d = new Date(s);
  return isNaN(d.getTime()) ? null : d;
}

// "just now", "4m ago", "2h ago", "3d ago", then a plain date.
export function relativeTime(raw: string): string {
  const d = parseTimestamp(raw);
  if (!d) return raw;
  const secs = Math.floor((Date.now() - d.getTime()) / 1000);
  if (secs < 45) return 'just now';
  if (secs < 90) return '1m ago';
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 14) return `${days}d ago`;
  return d.toLocaleDateString();
}

// Exact local form for tooltips.
export function exactTime(raw: string): string {
  const d = parseTimestamp(raw);
  return d ? d.toLocaleString() : raw;
}

// "12s", "1m 04s" between two timestamps; empty when either is missing.
export function duration(start: string, end: string | null | undefined): string {
  const a = parseTimestamp(start);
  const b = end ? parseTimestamp(end) : null;
  if (!a || !b) return '';
  const secs = Math.max(0, Math.round((b.getTime() - a.getTime()) / 1000));
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  return `${mins}m ${String(secs % 60).padStart(2, '0')}s`;
}
