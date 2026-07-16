import { useState, useEffect, type ReactNode } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { clearToken, getSettings, subscribeConnectivity } from '../api';
import { getThemePref, applyTheme, nextThemePref, type ThemePref } from '../lib/theme';
import { SunIcon, MoonIcon, MonitorIcon } from './Icons';
import ConfirmDialog from './ConfirmDialog';

/*
The admin initial barely changes; fetch it once per session instead of
once per page mount.
*/
let cachedInitial: string | null = null;

const themeIcons = {
  dark: <MoonIcon />,
  light: <SunIcon />,
  system: <MonitorIcon />,
};

const themeLabels: Record<ThemePref, string> = {
  dark: 'Theme: dark',
  light: 'Theme: light',
  system: 'Theme: follows your OS',
};

interface LayoutProps {
  title?: string;
  // Rendered dimmed after the nav links, e.g. the app name on its detail page.
  crumb?: string;
  children: ReactNode;
}

/*
Layout is the one shell every authenticated page shares: brand, nav
links with route-derived active state, theme toggle, New app button,
and the avatar/logout flow. It also owns document.title.
*/
export default function Layout({ title, crumb, children }: LayoutProps) {
  const [initial, setInitial] = useState(cachedInitial || '');
  const [confirmingLogout, setConfirmingLogout] = useState(false);
  const [theme, setTheme] = useState<ThemePref>(getThemePref);
  const [offline, setOffline] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    subscribeConnectivity(setOffline);
    return () => subscribeConnectivity(null);
  }, []);

  useEffect(() => {
    document.title = title ? `${title} · flightdeck` : 'flightdeck';
    return () => { document.title = 'flightdeck'; };
  }, [title]);

  useEffect(() => {
    if (cachedInitial !== null) return;
    getSettings()
      .then(s => {
        cachedInitial = s.admin_username.charAt(0);
        setInitial(cachedInitial);
      })
      .catch(() => { /* transient */ });
  }, []);

  function cycleTheme() {
    const next = nextThemePref(theme);
    applyTheme(next);
    setTheme(next);
  }

  function handleLogout() {
    cachedInitial = null;
    clearToken();
    navigate('/login');
  }

  const active = (path: string) =>
    `nav-link ${location.pathname === path ? 'nav-link-active' : ''}`;

  return (
    <div className="layout">
      <nav className="nav">
        <div className="nav-left">
          <Link to="/" className="nav-brand">flightdeck</Link>
          <div className="nav-links">
            <Link to="/" className={active('/')}>Apps</Link>
            <Link to="/settings" className={active('/settings')}>Settings</Link>
          </div>
          {crumb && <span className="nav-crumb">/ {crumb}</span>}
        </div>
        <div className="nav-right">
          <button
            className="btn btn-ghost btn-icon btn-sm"
            onClick={cycleTheme}
            title={themeLabels[theme]}
            aria-label={themeLabels[theme]}
          >
            {themeIcons[theme]}
          </button>
          <Link to="/deploy" className="btn btn-primary btn-sm nav-new-app">New app</Link>
          <button className="nav-avatar" onClick={() => setConfirmingLogout(true)} title="Log out" aria-label="Log out">
            {initial || '?'}
          </button>
        </div>
      </nav>

      {offline && (
        <div className="offline-banner" role="alert">
          Can't reach the server. Retrying in the background; data may be stale.
        </div>
      )}

      <main>{children}</main>

      <ConfirmDialog
        open={confirmingLogout}
        title="Log out?"
        message="Your apps keep running — this only signs you out of the dashboard."
        confirmLabel="Log out"
        onConfirm={handleLogout}
        onCancel={() => setConfirmingLogout(false)}
      />
    </div>
  );
}
