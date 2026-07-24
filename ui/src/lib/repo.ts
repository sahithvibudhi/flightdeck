/*
Human label for a repo URL. GitHub URLs become owner/repo; anything
else drops the scheme and keeps the host plus the tail of the path.
The full URL belongs in a tooltip next to this label.
*/
export function repoLabel(url: string): string {
  const gh = url.match(/^https?:\/\/(?:www\.)?github\.com\/([^/]+\/[^/]+?)(?:\.git)?\/?$/);
  if (gh) return gh[1];
  const stripped = url.replace(/^[a-z+]+:\/\//i, '').replace(/\.git$/, '').replace(/\/+$/, '');
  const parts = stripped.split('/').filter(Boolean);
  if (parts.length <= 2) return stripped;
  return `${parts[0]}/…/${parts[parts.length - 1]}`;
}
