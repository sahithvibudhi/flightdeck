import type { EnvVar } from '../api';

/*
Parse pasted .env content: KEY=VALUE lines, comments and blanks
skipped, surrounding quotes stripped. Returns the parsed pairs in
order; the caller decides how to merge them.
*/
export function parseEnvText(text: string): EnvVar[] {
  const out: EnvVar[] = [];
  for (const raw of text.split('\n')) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const eq = line.indexOf('=');
    if (eq <= 0) continue;
    const key = line.slice(0, eq).trim();
    let value = line.slice(eq + 1).trim();
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    out.push({ key, value });
  }
  return out;
}

// Merge parsed pairs over an existing list; later keys win.
export function mergeEnvs(current: EnvVar[], imported: EnvVar[]): EnvVar[] {
  const merged = new Map(current.filter(e => e.key.trim()).map(e => [e.key, e.value]));
  for (const { key, value } of imported) merged.set(key, value);
  return Array.from(merged, ([key, value]) => ({ key, value }));
}
