export type ThemePref = 'dark' | 'light' | 'system';

const KEY = 'theme';

export function getThemePref(): ThemePref {
  const v = localStorage.getItem(KEY);
  return v === 'dark' || v === 'light' ? v : 'system';
}

/*
The stylesheet handles all three states: an explicit data-theme wins,
otherwise the OS preference applies. "system" just removes the override.
*/
export function applyTheme(pref: ThemePref) {
  if (pref === 'system') {
    localStorage.removeItem(KEY);
    document.documentElement.removeAttribute('data-theme');
  } else {
    localStorage.setItem(KEY, pref);
    document.documentElement.setAttribute('data-theme', pref);
  }
}

export function initTheme() {
  applyTheme(getThemePref());
}

export function nextThemePref(current: ThemePref): ThemePref {
  if (current === 'system') return 'dark';
  if (current === 'dark') return 'light';
  return 'system';
}
