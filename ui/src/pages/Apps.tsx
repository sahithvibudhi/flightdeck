import { useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  listApps, clearToken, getSystemInfo, getServerMetrics,
  type App, type SystemInfo, type ServerMetricsHistory,
} from '../api';

function repoShortName(url: string): string {
  return url.replace(/^https?:\/\/(www\.)?github\.com\//, '').replace(/\.git$/, '');
}

function formatMemory(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  if (mb > 0) return `${mb.toFixed(1)} MB`;
  return '—';
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
  const navigate = useNavigate();

  useEffect(() => {
    loadAll();
    const interval = setInterval(loadAll, 5000);
    return () => clearInterval(interval);
  }, []);

  async function loadAll() {
    await Promise.all([loadApps(), loadSystem(), loadMetrics()]);
  }

  async function loadApps() {
    try { setApps(await listApps()); } catch {}
  }

  async function loadSystem() {
    try { setSystem(await getSystemInfo()); } catch {}
  }

  async function loadMetrics() {
    try { setMetrics(await getServerMetrics()); } catch {}
  }

  function handleLogout() {
    clearToken();
    navigate('/login');
  }

  const latest = metrics?.snapshots?.[metrics.snapshots.length - 1];

  return (
    <div className="layout">
      <nav className="nav">
        <Link to="/" className="nav-brand">nestops</Link>
        <div className="nav-links">
          <Link to="/settings">Settings</Link>
          <a href="#" onClick={handleLogout}>Log out</a>
        </div>
      </nav>
      <div className="container">

        {latest && metrics && (
          <div className="server-overview fade-in">
            <div className="server-stat">
              <div className="server-stat-header">
                <span className="server-stat-value">{latest.cpu_percent.toFixed(0)}%</span>
                <span className="server-stat-label">CPU</span>
              </div>
              <Sparkline data={metrics.snapshots.map(s => s.cpu_percent)} max={100} />
            </div>
            <div className="server-stat">
              <div className="server-stat-header">
                <span className="server-stat-value">{formatMemory(latest.memory_used_mb)}</span>
                <span className="server-stat-label">Memory</span>
              </div>
              <Sparkline data={metrics.snapshots.map(s => s.memory_used_mb)} max={latest.memory_total_mb} />
            </div>
            <div className="server-stat">
              <div className="server-stat-header">
                <span className="server-stat-value">{formatMemory(latest.disk_used_mb)}</span>
                <span className="server-stat-label">Disk</span>
              </div>
              <Sparkline data={metrics.snapshots.map(s => s.disk_used_mb)} max={latest.disk_total_mb} />
            </div>
            <div className="server-stat">
              <div className="server-stat-header">
                <span className="server-stat-value">{apps.filter(a => a.status === 'running').length}/{apps.length}</span>
                <span className="server-stat-label">Apps</span>
              </div>
              {system && (
                <div className="runtime-bar-items" style={{ marginTop: 4 }}>
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
          <Link to="/new" className="btn btn-primary btn-sm" style={{ textDecoration: 'none' }}>
            New app
          </Link>
        </div>

        {apps.length > 0 ? (
          <div className="app-grid">
            {apps.map(app => (
              <Link to={`/apps/${app.id}`} key={app.id} className="app-card">
                <div className="app-card-header">
                  <span className="app-card-name">{app.name}</span>
                  <span className={`badge badge-${app.status}`}>{app.status}</span>
                </div>

                {app.repo_url && (
                  <div className="app-card-repo">{repoShortName(app.repo_url)}</div>
                )}

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
            <p>No deployments yet</p>
            <Link to="/new" className="btn btn-primary btn-sm" style={{ textDecoration: 'none' }}>
              Deploy your first app
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}
