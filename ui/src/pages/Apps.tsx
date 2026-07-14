import { useState, useEffect, useCallback } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import {
  listApps, clearToken, getSystemInfo, getServerMetrics, getSettings,
  type App, type SystemInfo, type ServerMetricsHistory,
} from '../api';
import ConfirmDialog from '../components/ConfirmDialog';

function repoShortName(url: string): string {
  return url.replace(/^https?:\/\/(www\.)?github\.com\//, '').replace(/\.git$/, '');
}

function formatMemory(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  if (mb > 0) return `${mb.toFixed(1)} MB`;
  return '—';
}

function formatCapacity(usedMb: number, totalMb: number): { numbers: string; unit: string } {
  if (totalMb >= 1024) {
    return { numbers: `${(usedMb / 1024).toFixed(1)} / ${(totalMb / 1024).toFixed(1)}`, unit: 'GB' };
  }
  return { numbers: `${usedMb.toFixed(0)} / ${totalMb.toFixed(0)}`, unit: 'MB' };
}

function Sparkline({ data, max }: { data: number[]; max?: number }) {
  const ceil = max || Math.max(...data, 1);
  const bars = data.slice(-30);
  return (
    <div className="sparkline">
      {bars.map((v, i) => (
        <div
          key={i}
          className={`sparkline-bar ${i === bars.length - 1 ? 'sparkline-bar-active' : ''}`}
          style={{ height: `${Math.max((v / ceil) * 100, 2)}%` }}
        />
      ))}
    </div>
  );
}

export default function Apps() {
  const [apps, setApps] = useState<App[]>([]);
  const [system, setSystem] = useState<SystemInfo | null>(null);
  const [metrics, setMetrics] = useState<ServerMetricsHistory | null>(null);
  const [initial, setInitial] = useState('');
  const [confirmingLogout, setConfirmingLogout] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  const loadAll = useCallback(async () => {
    await Promise.all([
      listApps().then(setApps).catch(() => { /* transient */ }),
      getSystemInfo().then(setSystem).catch(() => { /* transient */ }),
      getServerMetrics().then(setMetrics).catch(() => { /* transient */ }),
    ]);
  }, []);

  useEffect(() => {
    loadAll();
    getSettings().then(s => setInitial(s.admin_username.charAt(0))).catch(() => { /* transient */ });
    const interval = setInterval(loadAll, 5000);
    return () => clearInterval(interval);
  }, [loadAll]);

  function handleLogout() {
    clearToken();
    navigate('/login');
  }

  const latest = metrics?.snapshots?.[metrics.snapshots.length - 1];

  return (
    <div className="layout">
      <nav className="nav">
        <div className="nav-left">
          <Link to="/" className="nav-brand">flightdeck</Link>
          <div className="nav-links">
            <Link to="/" className={`nav-link ${location.pathname === '/' ? 'nav-link-active' : ''}`}>Apps</Link>
            <Link to="/settings" className="nav-link">Settings</Link>
          </div>
        </div>
        <div className="nav-right">
          <Link to="/deploy" className="btn btn-primary btn-sm" style={{ textDecoration: 'none' }}>New app</Link>
          <button className="nav-avatar" onClick={() => setConfirmingLogout(true)} title="Log out" aria-label="Log out">{initial || '?'}</button>
        </div>
      </nav>
      <div className="container">

        {latest && metrics && (
          <div className="server-overview fade-in">
            <div className="server-stat">
              <span className="server-stat-label">CPU</span>
              <span className="server-stat-value">{latest.cpu_percent.toFixed(0)}<span className="server-stat-unit">%</span></span>
              <Sparkline data={metrics.snapshots.map(s => s.cpu_percent)} max={100} />
            </div>
            <div className="server-stat">
              <span className="server-stat-label">Memory <span className="server-stat-unit-inline">{formatCapacity(latest.memory_used_mb, latest.memory_total_mb).unit}</span></span>
              <span className="server-stat-value">{formatCapacity(latest.memory_used_mb, latest.memory_total_mb).numbers}</span>
              <span className="server-stat-pct">{(latest.memory_used_mb / latest.memory_total_mb * 100).toFixed(0)}% used</span>
              <Sparkline data={metrics.snapshots.map(s => s.memory_used_mb)} max={latest.memory_total_mb} />
            </div>
            <div className="server-stat">
              <span className="server-stat-label">Disk <span className="server-stat-unit-inline">{formatCapacity(latest.disk_used_mb, latest.disk_total_mb).unit}</span></span>
              <span className="server-stat-value">{formatCapacity(latest.disk_used_mb, latest.disk_total_mb).numbers}</span>
              <span className="server-stat-pct">{(latest.disk_used_mb / latest.disk_total_mb * 100).toFixed(0)}% used</span>
              <Sparkline data={metrics.snapshots.map(s => s.disk_used_mb)} max={latest.disk_total_mb} />
            </div>
            <div className="server-stat">
              <span className="server-stat-label">Apps</span>
              <span className="server-stat-value">{apps.filter(a => a.status === 'running').length}<span className="server-stat-unit">/{apps.length}</span></span>
              {system && (
                <div className="runtime-bar-items" style={{ marginTop: 4 }}>
                  <span className={`runtime-pill ${system.caddy.running ? 'runtime-pill-ok' : 'runtime-pill-down'}`} style={{ fontSize: 10, padding: '0 6px' }}>
                    <span className="runtime-dot" />
                    Caddy
                  </span>
                  {system.runtimes.filter(r => r.installed).map(r => (
                    <span key={r.name} className="runtime-pill runtime-pill-ok" style={{ fontSize: 10, padding: '0 6px' }}>
                      <span className="runtime-dot" />
                      {r.name}
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        <div className="page-header">
          <span className="page-title">Deployments</span>
        </div>

        {apps.length > 0 ? (
          <div className="app-grid">
            {apps.map(app => (
              <Link to={`/apps/${app.id}`} key={app.id} className="app-card">
                <div className="app-card-header">
                  <span className="app-card-name">{app.name}</span>
                  <span className={`badge badge-${app.status}`}>{app.status}</span>
                </div>

                <div className={`app-card-repo ${!app.repo_url ? 'app-card-repo-none' : ''}`}>
                  {app.repo_url ? repoShortName(app.repo_url) : 'No repository'}
                </div>

                <div className="app-card-metrics">
                  <div className="app-card-metric">
                    <span className="app-card-metric-value">
                      {app.status === 'running' ? `${app.cpu_percent}%` : '—'}
                    </span>
                    <span className="app-card-metric-label">CPU</span>
                  </div>
                  <div className="app-card-metric">
                    <span className="app-card-metric-value">
                      {app.status === 'running' ? formatMemory(app.memory_mb) : '—'}
                    </span>
                    <span className="app-card-metric-label">Memory</span>
                  </div>
                  <div className="app-card-metric">
                    <span className="app-card-metric-value">:{app.port}</span>
                    <span className="app-card-metric-label">Port</span>
                  </div>
                </div>

                {app.domains.length > 0 && (
                  <div className="app-card-domains">
                    {app.domains.map(d => (
                      <span key={d} className="app-card-domain">{d}</span>
                    ))}
                  </div>
                )}
              </Link>
            ))}
          </div>
        ) : (
          <div className="empty-state">
            <p style={{ fontSize: 15, marginBottom: 8 }}>Your server is ready</p>
            <p>Three steps from here to a live app:</p>
            <ol className="getting-started">
              <li><span className="getting-started-num">1</span> Deploy an app from GitHub, a zip file, or a directory on this server</li>
              <li><span className="getting-started-num">2</span> Point a DNS record here and add the domain — SSL is automatic</li>
              <li><span className="getting-started-num">3</span> Paste the app's webhook URL into GitHub for push-to-deploy</li>
            </ol>
            <Link to="/deploy" className="btn btn-primary btn-sm" style={{ textDecoration: 'none' }}>
              Deploy your first app
            </Link>
            <Link to="/settings" className="btn-text" style={{ textDecoration: 'none', marginTop: 12, fontSize: 12 }}>
              or set up runtimes and tokens in Settings
            </Link>
          </div>
        )}
      </div>

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
