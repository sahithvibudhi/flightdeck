import { useState, useEffect, useCallback, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  listApps, getSystemInfo, getServerMetrics, createSampleApp, errMsg,
  type App, type SystemInfo, type ServerMetricsHistory,
} from '../api';
import { relativeTime, exactTime, parseTimestamp } from '../lib/time';
import Layout from '../components/Layout';

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

function percentUsed(used: number, total: number): string {
  if (total <= 0) return '';
  return `${(used / total * 100).toFixed(0)}% used`;
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

type SortKey = 'name' | 'status' | 'deployed';

const statusRank: Record<string, number> = {
  crashed: 0, error: 1, restarting: 2, deploying: 3, running: 4, stopped: 5,
};

export default function Apps() {
  const [apps, setApps] = useState<App[]>([]);
  const [system, setSystem] = useState<SystemInfo | null>(null);
  const [metrics, setMetrics] = useState<ServerMetricsHistory | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState<SortKey>('name');
  const [sampleLoading, setSampleLoading] = useState(false);
  const [sampleError, setSampleError] = useState('');
  const navigate = useNavigate();

  const loadAll = useCallback(async () => {
    await Promise.all([
      listApps().then(setApps).catch(() => { /* transient */ }),
      getSystemInfo().then(setSystem).catch(() => { /* transient */ }),
      getServerMetrics().then(setMetrics).catch(() => { /* transient */ }),
    ]);
    setLoaded(true);
  }, []);

  useEffect(() => {
    loadAll();
    const interval = setInterval(loadAll, 5000);
    return () => clearInterval(interval);
  }, [loadAll]);

  async function handleDeploySample() {
    setSampleLoading(true);
    setSampleError('');
    try {
      const app = await createSampleApp();
      navigate(`/apps/${app.id}`);
    } catch (err) {
      setSampleError(errMsg(err));
    } finally {
      setSampleLoading(false);
    }
  }

  const visibleApps = useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = q
      ? apps.filter(a =>
          a.name.toLowerCase().includes(q) ||
          (a.repo_url || '').toLowerCase().includes(q) ||
          a.domains.some(d => d.toLowerCase().includes(q)))
      : apps;
    return [...filtered].sort((a, b) => {
      if (sort === 'status') {
        return (statusRank[a.status] ?? 9) - (statusRank[b.status] ?? 9) || a.name.localeCompare(b.name);
      }
      if (sort === 'deployed') {
        const ta = parseTimestamp(a.last_deploy_at || '')?.getTime() ?? 0;
        const tb = parseTimestamp(b.last_deploy_at || '')?.getTime() ?? 0;
        return tb - ta || a.name.localeCompare(b.name);
      }
      return a.name.localeCompare(b.name);
    });
  }, [apps, query, sort]);

  const latest = metrics?.snapshots?.[metrics.snapshots.length - 1];

  return (
    <Layout>
      <div className="container">

        {system && !system.caddy.running && (
          <div className="warning-banner fade-in">
            <span>
              Caddy isn't running — domains and automatic SSL are disabled.
            </span>
            <Link to="/settings" className="warning-banner-link">Install from Settings →</Link>
          </div>
        )}

        {!loaded ? (
          <div className="server-overview">
            {[0, 1, 2, 3].map(i => <div key={i} className="skeleton skeleton-stat" />)}
          </div>
        ) : latest && metrics && (
          <div className="server-overview fade-in">
            <div className="server-stat">
              <span className="server-stat-label">CPU</span>
              <span className="server-stat-value">{latest.cpu_percent.toFixed(0)}<span className="server-stat-unit">%</span></span>
              <Sparkline data={metrics.snapshots.map(s => s.cpu_percent)} max={100} />
            </div>
            <div className="server-stat">
              <span className="server-stat-label">Memory <span className="server-stat-unit-inline">{formatCapacity(latest.memory_used_mb, latest.memory_total_mb).unit}</span></span>
              <span className="server-stat-value">{formatCapacity(latest.memory_used_mb, latest.memory_total_mb).numbers}</span>
              <span className="server-stat-pct">{percentUsed(latest.memory_used_mb, latest.memory_total_mb)}</span>
              <Sparkline data={metrics.snapshots.map(s => s.memory_used_mb)} max={latest.memory_total_mb} />
            </div>
            <div className="server-stat">
              <span className="server-stat-label">Disk <span className="server-stat-unit-inline">{formatCapacity(latest.disk_used_mb, latest.disk_total_mb).unit}</span></span>
              <span className="server-stat-value">{formatCapacity(latest.disk_used_mb, latest.disk_total_mb).numbers}</span>
              <span className="server-stat-pct">{percentUsed(latest.disk_used_mb, latest.disk_total_mb)}</span>
              <Sparkline data={metrics.snapshots.map(s => s.disk_used_mb)} max={latest.disk_total_mb} />
            </div>
            <div className="server-stat">
              <span className="server-stat-label">Apps</span>
              <span className="server-stat-value">{apps.filter(a => a.status === 'running').length}<span className="server-stat-unit">/{apps.length}</span></span>
              {system && (
                <div className="runtime-bar-items" style={{ marginTop: 4 }}>
                  <span className={`runtime-pill runtime-pill-compact ${system.caddy.running ? 'runtime-pill-ok' : 'runtime-pill-down'}`}>
                    <span className="runtime-dot" />
                    Caddy
                  </span>
                  {system.runtimes.filter(r => r.installed).map(r => (
                    <span key={r.name} className="runtime-pill runtime-pill-compact runtime-pill-ok">
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
          <span className="page-title">Apps</span>
          {apps.length > 1 && (
            <div className="flex-center gap-sm">
              <input
                type="search"
                className="app-search"
                placeholder="Search apps"
                aria-label="Search apps"
                value={query}
                onChange={e => setQuery(e.target.value)}
              />
              <select
                className="app-sort"
                aria-label="Sort apps"
                value={sort}
                onChange={e => setSort(e.target.value as SortKey)}
              >
                <option value="name">Sort: name</option>
                <option value="status">Sort: status</option>
                <option value="deployed">Sort: last deploy</option>
              </select>
            </div>
          )}
        </div>

        {!loaded ? (
          <div className="app-grid">
            {[0, 1, 2].map(i => <div key={i} className="skeleton skeleton-card" />)}
          </div>
        ) : apps.length > 0 ? (
          visibleApps.length > 0 ? (
            <div className="app-grid">
              {visibleApps.map(app => (
                <div key={app.id} className="app-card">
                  <div className="app-card-header">
                    {/* Stretched link: the name anchor covers the card. */}
                    <Link to={`/apps/${app.id}`} className="app-card-name app-card-link">{app.name}</Link>
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
                      <span className="app-card-metric-value">:{app.url_port}</span>
                      <span className="app-card-metric-label">Port</span>
                    </div>
                  </div>

                  <div className="app-card-footer">
                    {app.domains.length > 0 ? (
                      <div className="app-card-domains">
                        {app.domains.map(d => (
                          <a
                            key={d}
                            className="app-card-domain app-card-domain-link"
                            href={`https://${d}`}
                            target="_blank"
                            rel="noreferrer"
                          >
                            {d}
                          </a>
                        ))}
                      </div>
                    ) : (app.status === 'running' && system?.server_ip ? (
                      <div className="app-card-domains">
                        <a
                          className="app-card-domain app-card-domain-link"
                          href={`http://${system.server_ip}:${app.url_port}`}
                          target="_blank"
                          rel="noreferrer"
                        >
                          {system.server_ip}:{app.url_port}
                        </a>
                      </div>
                    ) : <span />)}
                    {app.last_deploy_at && (
                      <span
                        className={`app-card-deployed ${app.last_deploy_status === 'failed' ? 'app-card-deployed-failed' : ''}`}
                        title={exactTime(app.last_deploy_at)}
                      >
                        {app.last_deploy_status === 'failed' ? 'failed deploy' : 'deployed'} {relativeTime(app.last_deploy_at)}
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="list-empty">No apps match "{query}".</p>
          )
        ) : (
          <div className="empty-state">
            <p style={{ fontSize: 15, marginBottom: 8 }}>Your server is ready</p>
            <p>Three steps from here to a live app:</p>
            <ol className="getting-started">
              <li><span className="getting-started-num">1</span> Deploy an app from GitHub, a zip file, or a directory on this server</li>
              <li><span className="getting-started-num">2</span> Point a DNS record here and add the domain — SSL is automatic</li>
              <li><span className="getting-started-num">3</span> Paste the app's webhook URL into GitHub for push-to-deploy</li>
            </ol>
            <Link to="/deploy" className="btn btn-primary btn-sm">
              Deploy your first app
            </Link>
            <button className="btn btn-secondary btn-sm" style={{ marginTop: 12 }} onClick={handleDeploySample} disabled={sampleLoading}>
              {sampleLoading ? <span className="spinner" /> : 'Deploy a sample app instead'}
            </button>
            {sampleError && <p className="error-msg">{sampleError}</p>}
            <Link to="/settings" className="btn-text" style={{ marginTop: 12, fontSize: 12 }}>
              or set up runtimes and tokens in Settings
            </Link>
          </div>
        )}
      </div>
    </Layout>
  );
}
