import { useState, useEffect, useRef, useCallback, type FormEvent } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import {
  getApp, deleteApp, startApp, stopApp, restartApp, pullApp, updateApp,
  getAppLogs, streamAppLogs, listEnvs, replaceEnvs, listDomains, addDomain, removeDomain,
  listDeployments, deployApp, rollbackDeployment, getDeploymentLogs, streamDeploymentLogs,
  getSystemInfo, errMsg, findInvalidEnv,
  type App, type EnvVar, type DomainEntry, type Deployment, type SystemInfo,
} from '../api';
import { EyeIcon, EyeOffIcon, ExternalLinkIcon } from '../components/Icons';
import { toast } from '../components/toastBus';
import { relativeTime, exactTime, duration } from '../lib/time';
import ConfirmDialog from '../components/ConfirmDialog';
import Layout from '../components/Layout';
import LogViewer from '../components/LogViewer';

function formatMemory(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  if (mb > 0) return `${mb.toFixed(1)} MB`;
  return '—';
}

type Tab = 'overview' | 'logs' | 'deployments' | 'configuration';

interface DeployPanel {
  depId: string;
  lines: string[];
  running: boolean;
}

export default function AppDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [app, setApp] = useState<App | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [tab, setTab] = useState<Tab>('overview');
  const [logs, setLogs] = useState<string[]>([]);
  const [envs, setEnvs] = useState<EnvVar[]>([]);
  const [domains, setDomains] = useState<DomainEntry[]>([]);
  const [newDomain, setNewDomain] = useState('');
  const [envError, setEnvError] = useState('');
  const [domainError, setDomainError] = useState('');
  const [configError, setConfigError] = useState('');
  const [envsDirtySince, setEnvsDirtySince] = useState(false);
  const [pulling, setPulling] = useState(false);
  const [pullOutput, setPullOutput] = useState('');
  const [actionLoading, setActionLoading] = useState('');
  const [editing, setEditing] = useState(false);
  const [editName, setEditName] = useState('');
  const [editStartCmd, setEditStartCmd] = useState('');
  const [editBuildCmd, setEditBuildCmd] = useState('');
  const [editPort, setEditPort] = useState('');
  const [editRepoUrl, setEditRepoUrl] = useState('');
  const [editBranch, setEditBranch] = useState('');
  const [editWorkDir, setEditWorkDir] = useState('');
  const [editHealthPath, setEditHealthPath] = useState('');
  const [shownEnvValues, setShownEnvValues] = useState<Set<number>>(new Set());
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [expandedDep, setExpandedDep] = useState<string | null>(null);
  const [expandedDepLines, setExpandedDepLines] = useState<string[]>([]);
  const [copied, setCopied] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [confirmingAction, setConfirmingAction] = useState<'stop' | 'restart' | null>(null);
  const [rollbackTarget, setRollbackTarget] = useState<Deployment | null>(null);
  const [system, setSystem] = useState<SystemInfo | null>(null);
  const [deployPanel, setDeployPanel] = useState<DeployPanel | null>(null);
  const deployStreamRef = useRef<EventSource | null>(null);

  useEffect(() => {
    getSystemInfo().then(setSystem).catch(() => { /* transient */ });
  }, []);

  const loadApp = useCallback(async () => {
    try {
      setApp(await getApp(id!));
    } catch (err) {
      if (errMsg(err).toLowerCase().includes('not found')) setNotFound(true);
    }
  }, [id]);

  const loadLogs = useCallback(async () => {
    try {
      const res = await getAppLogs(id!);
      setLogs(res.lines);
    } catch { /* transient */ }
  }, [id]);

  const loadEnvs = useCallback(async () => {
    try { setEnvs(await listEnvs(id!)); } catch { /* transient */ }
  }, [id]);

  const loadDomains = useCallback(async () => {
    try { setDomains(await listDomains(id!)); } catch { /* transient */ }
  }, [id]);

  const loadDeployments = useCallback(async () => {
    try { setDeployments(await listDeployments(id!)); } catch { /* transient */ }
  }, [id]);

  useEffect(() => {
    if (!id) return;
    loadApp();
    loadEnvs();
    loadDomains();
    loadDeployments();
    const metricInterval = setInterval(() => {
      loadApp();
      loadDeployments();
    }, 5000);
    return () => clearInterval(metricInterval);
  }, [id, loadApp, loadEnvs, loadDomains, loadDeployments]);

  /*
  Live deploy panel. followDeploy attaches to a deployment's SSE log
  stream; it is used both for deploys started here and for ones that
  arrive from outside (webhook pushes), which the poller below spots.
  */
  const followDeploy = useCallback((depId: string) => {
    deployStreamRef.current?.close();
    setDeployPanel({ depId, lines: [], running: true });
    const source = streamDeploymentLogs(id!, depId);
    deployStreamRef.current = source;

    source.onmessage = e => {
      setDeployPanel(prev =>
        prev && prev.depId === depId
          ? { ...prev, lines: [...prev.lines.slice(-999), e.data] }
          : prev
      );
    };
    source.addEventListener('done', () => {
      source.close();
      setDeployPanel(prev => (prev && prev.depId === depId ? { ...prev, running: false } : prev));
      Promise.all([loadApp(), loadDeployments(), loadLogs()]).then(() => {
        listDeployments(id!).then(deps => {
          const dep = deps.find(d => d.id === depId);
          if (dep?.status === 'success') toast('Deploy succeeded');
          else if (dep?.status === 'failed') toast('Deploy failed. Check the deploy log.', 'error');
        }).catch(() => { /* transient */ });
      });
    });
    source.onerror = () => {
      // Stream dropped (proxy, restart); the poller keeps history fresh.
      source.close();
      setDeployPanel(prev => (prev && prev.depId === depId ? { ...prev, running: false } : prev));
    };
  }, [id, loadApp, loadDeployments, loadLogs]);

  useEffect(() => () => deployStreamRef.current?.close(), []);

  // Attach to deploys started elsewhere (webhooks, another tab).
  useEffect(() => {
    const running = deployments.find(d => d.status === 'running');
    if (running && (!deployPanel || (deployPanel.depId !== running.id && !deployPanel.running))) {
      followDeploy(running.id);
    }
  }, [deployments, deployPanel, followDeploy]);

  // App runtime logs stream live over SSE; fall back to polling if the
  // stream can't connect.
  useEffect(() => {
    if (!id) return;
    let pollInterval: ReturnType<typeof setInterval> | null = null;
    const source = streamAppLogs(id);

    source.onmessage = e => {
      setLogs(prev => [...prev.slice(-499), e.data]);
    };
    source.onerror = () => {
      source.close();
      if (!pollInterval) {
        loadLogs();
        pollInterval = setInterval(loadLogs, 3000);
      }
    };

    return () => {
      source.close();
      if (pollInterval) clearInterval(pollInterval);
    };
  }, [id, loadLogs]);

  async function handleAction(label: string, action: () => Promise<unknown>) {
    setActionLoading(label);
    try {
      await action();
      await loadApp();
    } catch (err) {
      toast(errMsg(err), 'error');
    } finally {
      setActionLoading('');
    }
  }

  async function handlePull() {
    setPullOutput('');
    setPulling(true);
    try {
      const res = await pullApp(id!);
      setPullOutput(res.output);
    } catch (err) {
      toast(errMsg(err), 'error');
    } finally {
      setPulling(false);
    }
  }

  async function handleDeploy() {
    try {
      const res = await deployApp(id!);
      if (res.deployment_id) followDeploy(res.deployment_id);
      await loadDeployments();
    } catch (err) {
      toast(errMsg(err), 'error');
    }
  }

  async function handleRollback() {
    if (!rollbackTarget) return;
    const target = rollbackTarget;
    setRollbackTarget(null);
    try {
      const res = await rollbackDeployment(id!, target.id);
      if (res.deployment_id) followDeploy(res.deployment_id);
      await loadDeployments();
    } catch (err) {
      toast(errMsg(err), 'error');
    }
  }

  async function toggleDeploymentLog(depId: string) {
    if (expandedDep === depId) {
      setExpandedDep(null);
      return;
    }
    try {
      const res = await getDeploymentLogs(id!, depId);
      setExpandedDepLines(res.lines);
      setExpandedDep(depId);
    } catch (err) {
      toast(errMsg(err), 'error');
    }
  }

  async function copyWebhookUrl() {
    if (!app) return;
    const url = `${window.location.origin}/hooks/${app.id}?secret=${app.webhook_secret}`;
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast('Could not copy. Copy the URL manually.', 'error');
    }
  }

  async function handleDelete() {
    setConfirmingDelete(false);
    try {
      await deleteApp(id!);
      navigate('/');
    } catch (err) {
      toast(errMsg(err), 'error');
    }
  }

  function updateEnv(index: number, field: 'key' | 'value', val: string) {
    const updated = [...envs];
    updated[index] = { ...updated[index], [field]: val };
    setEnvs(updated);
  }

  function addEnvRow() {
    setEnvs([...envs, { key: '', value: '' }]);
  }

  function removeEnvRow(index: number) {
    setEnvs(envs.filter((_, i) => i !== index));
  }

  /*
  Paste a whole .env file: KEY=VALUE lines are parsed and merged over
  the current list (later keys win), comments and blanks skipped.
  */
  function importEnvText() {
    const text = window.prompt('Paste .env contents (KEY=VALUE lines):');
    if (!text) return;
    const merged = new Map(envs.filter(e => e.key.trim()).map(e => [e.key, e.value]));
    let count = 0;
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
      merged.set(key, value);
      count++;
    }
    if (count === 0) {
      setEnvError('No KEY=VALUE lines found in the pasted text');
      return;
    }
    setEnvError('');
    setEnvs(Array.from(merged, ([key, value]) => ({ key, value })));
    toast(`Imported ${count} variable${count === 1 ? '' : 's'}. Review and save.`);
  }

  function toggleEnvVisibility(index: number) {
    setShownEnvValues(prev => {
      const next = new Set(prev);
      if (next.has(index)) {
        next.delete(index);
      } else {
        next.add(index);
      }
      return next;
    });
  }

  async function saveEnvs(e: FormEvent) {
    e.preventDefault();
    setEnvError('');
    const toSave = envs.filter(e => e.key.trim() !== '');
    const invalid = findInvalidEnv(toSave);
    if (invalid) {
      setEnvError(invalid);
      return;
    }
    try {
      await replaceEnvs(id!, toSave);
      await loadEnvs();
      if (app?.status === 'running') {
        setEnvsDirtySince(true);
        toast('Saved. Restart to apply the new environment.');
      } else {
        toast('Environment variables saved');
      }
    } catch (err) {
      setEnvError(errMsg(err));
    }
  }

  async function handleAddDomain(e: FormEvent) {
    e.preventDefault();
    const domain = newDomain.trim().toLowerCase();
    if (!domain) return;
    setDomainError('');
    try {
      await addDomain(id!, domain);
      setNewDomain('');
      await loadDomains();
      toast('Domain added — SSL certificate provisions automatically');
    } catch (err) {
      setDomainError(errMsg(err));
    }
  }

  async function handleRemoveDomain(domain: string) {
    setDomainError('');
    try {
      await removeDomain(id!, domain);
      await loadDomains();
      toast('Domain removed');
    } catch (err) {
      setDomainError(errMsg(err));
    }
  }

  function startEditing() {
    if (!app) return;
    setEditName(app.name);
    setEditStartCmd(app.start_command);
    setEditBuildCmd(app.build_command || '');
    setEditPort(String(app.port));
    setEditRepoUrl(app.repo_url || '');
    setEditBranch(app.branch || '');
    setEditWorkDir(app.work_dir || '');
    setEditHealthPath(app.health_path || '');
    setConfigError('');
    setEditing(true);
  }

  async function handleSaveConfig(e: FormEvent) {
    e.preventDefault();
    setConfigError('');
    try {
      await updateApp(id!, {
        name: editName,
        start_command: editStartCmd,
        build_command: editBuildCmd,
        port: parseInt(editPort, 10) || undefined,
        repo_url: editRepoUrl || undefined,
        branch: editBranch || undefined,
        work_dir: editWorkDir || undefined,
        health_path: editHealthPath,
      });
      setEditing(false);
      await loadApp();
      toast('Configuration saved');
    } catch (err) {
      setConfigError(errMsg(err));
    }
  }

  if (notFound) {
    return (
      <Layout title="Not found">
        <div className="container">
          <div className="empty-state fade-in">
            <p style={{ fontSize: 15, marginBottom: 8 }}>App not found</p>
            <p>It may have been deleted, or the link is stale.</p>
            <Link to="/" className="btn btn-primary btn-sm" style={{ textDecoration: 'none' }}>
              Back to apps
            </Link>
          </div>
        </div>
      </Layout>
    );
  }

  if (!app) {
    return (
      <Layout>
        <div className="container">
          <div className="flex-center gap-sm" style={{ padding: '40px 0', justifyContent: 'center' }}>
            <span className="spinner" />
          </div>
        </div>
      </Layout>
    );
  }

  const liveDeployId = deployments.find(d => d.status === 'success')?.id;

  const tabs: { key: Tab; label: string; count?: number }[] = [
    { key: 'overview', label: 'Overview' },
    { key: 'logs', label: 'Logs' },
    { key: 'deployments', label: 'Deployments', count: deployments.length },
    { key: 'configuration', label: 'Configuration' },
  ];

  return (
    <Layout title={app.name} crumb={app.name}>
      <div className="container fade-in">
        <div className="app-detail-header">
          <div className="app-detail-title">
            <div className="app-detail-name">{app.name}</div>
            <div className="app-detail-meta">
              <span className={`badge badge-${app.status}`}>{app.status}</span>
              <span>port {app.port}</span>
              {app.repo_url
                ? <span>{app.branch || 'main'}</span>
                : <span style={{ opacity: 0.5 }}>manual deploy</span>
              }
              {app.status === 'running' && (domains.length > 0 || system?.server_ip) && (
                <a
                  className="app-url"
                  href={domains.length > 0 ? `https://${domains[0].domain}` : `http://${system!.server_ip}:${app.url_port}`}
                  target="_blank"
                  rel="noreferrer"
                >
                  {domains.length > 0 ? domains[0].domain : `${system!.server_ip}:${app.url_port}`}
                  <ExternalLinkIcon />
                </a>
              )}
            </div>
          </div>
          <div className="app-actions">
            {app.repo_url && (
              <>
                <button
                  className="btn btn-primary btn-sm"
                  onClick={handleDeploy}
                  disabled={!!actionLoading || deployPanel?.running}
                >
                  Deploy
                </button>
                <button
                  className="btn btn-secondary btn-sm"
                  onClick={handlePull}
                  disabled={pulling}
                >
                  {pulling ? <><span className="spinner" /> Pulling...</> : 'Pull'}
                </button>
              </>
            )}
            {app.status !== 'running' ? (
              <button
                className="btn btn-primary btn-sm"
                onClick={() => handleAction('start', () => startApp(id!))}
                disabled={!!actionLoading}
              >
                {actionLoading === 'start' ? <span className="spinner" /> : 'Start'}
              </button>
            ) : (
              <button
                className="btn btn-secondary btn-sm"
                onClick={() => setConfirmingAction('stop')}
                disabled={!!actionLoading}
              >
                {actionLoading === 'stop' ? <span className="spinner" /> : 'Stop'}
              </button>
            )}
            <button
              className="btn btn-secondary btn-sm"
              onClick={() => {
                if (app.status === 'running') setConfirmingAction('restart');
                else handleAction('restart', () => restartApp(id!));
              }}
              disabled={!!actionLoading}
            >
              {actionLoading === 'restart' ? <span className="spinner" /> : 'Restart'}
            </button>
          </div>
        </div>

        {deployPanel && (
          <div className="section fade-in">
            <div className="section-header">
              <h2>
                {deployPanel.running ? 'Deploy in progress' : 'Last deploy'}
              </h2>
              <div className="flex-center gap-sm">
                {deployPanel.running && <span className="spinner" />}
                {!deployPanel.running && (
                  <button className="btn btn-ghost btn-sm" onClick={() => setDeployPanel(null)}>Dismiss</button>
                )}
              </div>
            </div>
            <LogViewer
              lines={deployPanel.lines}
              filename={`${app.name}-deploy.log`}
              emptyText="Waiting for deploy output..."
              compact
            />
          </div>
        )}

        <div className="tabs" role="tablist">
          {tabs.map(t => (
            <button
              key={t.key}
              role="tab"
              aria-selected={tab === t.key}
              className={`tab ${tab === t.key ? 'tab-active' : ''}`}
              onClick={() => setTab(t.key)}
            >
              {t.label}
              {t.count !== undefined && t.count > 0 && <span className="tab-count">{t.count}</span>}
            </button>
          ))}
        </div>

        {tab === 'overview' && (
          <>
            {app.status === 'running' ? (
              <div className="metrics-bar fade-in">
                <div className="metrics-bar-item">
                  <span className="metrics-bar-value">{app.cpu_percent}%</span>
                  <span className="metrics-bar-label">CPU</span>
                </div>
                <div className="metrics-bar-item">
                  <span className="metrics-bar-value">{formatMemory(app.memory_mb)}</span>
                  <span className="metrics-bar-label">Memory</span>
                </div>
                <div className="metrics-bar-item">
                  <span className="metrics-bar-value">:{app.url_port}</span>
                  <span className="metrics-bar-label">Serving on</span>
                </div>
              </div>
            ) : (
              <p className="list-empty">
                {app.status === 'crashed'
                  ? 'The app crashed repeatedly and gave up. Check the logs, fix the app, and press Start.'
                  : 'The app is not running. Start it to see live metrics.'}
              </p>
            )}

            {envsDirtySince && app.status === 'running' && (
              <div className="warning-banner">
                <span>Environment changes are saved but the running process still has the old values.</span>
                <button
                  className="btn btn-sm btn-secondary"
                  onClick={() => { setEnvsDirtySince(false); setConfirmingAction('restart'); }}
                >
                  Restart now
                </button>
              </div>
            )}

            {pullOutput && (
              <div className="pull-output fade-in">{pullOutput}</div>
            )}

            {deployments.length > 0 && (
              <div className="section mt-md">
                <h2>Latest deployment</h2>
                <DeploymentRow
                  d={deployments[0]}
                  isLive={deployments[0].id === liveDeployId}
                  expanded={expandedDep === deployments[0].id}
                  expandedLines={expandedDepLines}
                  appName={app.name}
                  onToggleLog={toggleDeploymentLog}
                  onRollback={setRollbackTarget}
                  canRollback={false}
                />
                <button className="btn btn-text mt-sm" onClick={() => setTab('deployments')}>
                  View all deployments
                </button>
              </div>
            )}
          </>
        )}

        {tab === 'logs' && (
          <LogViewer
            lines={logs}
            filename={`${app.name}.log`}
            emptyText="Waiting for output..."
            capNotice={logs.length >= 500 ? 'Showing the last 500 lines. Download for the full file.' : undefined}
          />
        )}

        {tab === 'deployments' && (
          deployments.length > 0 ? (
            <div className="section">
              {deployments.map(d => (
                <DeploymentRow
                  key={d.id}
                  d={d}
                  isLive={d.id === liveDeployId}
                  expanded={expandedDep === d.id}
                  expandedLines={expandedDepLines}
                  appName={app.name}
                  onToggleLog={toggleDeploymentLog}
                  onRollback={setRollbackTarget}
                  canRollback={d.status === 'success' && !!d.commit_sha && d.id !== liveDeployId}
                />
              ))}
            </div>
          ) : (
            <p className="list-empty">
              {app.repo_url
                ? 'No deployments yet. Press Deploy, or push to the repo with the webhook configured.'
                : 'No deployments yet. Deployments appear when the app is redeployed from git.'}
            </p>
          )
        )}

        {tab === 'configuration' && (
          <>
            <div className="section">
              <div className="section-header">
                <h2>Configuration</h2>
                {!editing && <button className="btn btn-secondary btn-sm" onClick={startEditing}>Edit</button>}
              </div>
              {editing ? (
                <form onSubmit={handleSaveConfig}>
                  <div className="config-grid">
                    <div className="form-group" style={{ marginBottom: 12 }}>
                      <label htmlFor="edit-name">App name</label>
                      <input id="edit-name" value={editName} onChange={e => setEditName(e.target.value)} />
                    </div>
                    <div className="form-group" style={{ marginBottom: 12 }}>
                      <label htmlFor="edit-start">Start command</label>
                      <input id="edit-start" value={editStartCmd} onChange={e => setEditStartCmd(e.target.value)} />
                    </div>
                    <div className="form-group" style={{ marginBottom: 12 }}>
                      <label htmlFor="edit-build">Build command</label>
                      <input id="edit-build" value={editBuildCmd} onChange={e => setEditBuildCmd(e.target.value)} placeholder="e.g. npm install" />
                    </div>
                    <div className="form-group" style={{ marginBottom: 12 }}>
                      <label htmlFor="edit-port">Port</label>
                      <input id="edit-port" value={editPort} onChange={e => setEditPort(e.target.value.replace(/\D/g, ''))} placeholder="e.g. 3000" />
                    </div>
                    <div className="form-group" style={{ marginBottom: 12 }}>
                      <label htmlFor="edit-repo">Repository URL</label>
                      <input id="edit-repo" value={editRepoUrl} onChange={e => setEditRepoUrl(e.target.value)} placeholder="https://github.com/..." />
                    </div>
                    <div className="form-group" style={{ marginBottom: 12 }}>
                      <label htmlFor="edit-branch">Branch</label>
                      <input id="edit-branch" value={editBranch} onChange={e => setEditBranch(e.target.value)} placeholder="main" />
                    </div>
                    {app.work_dir && (
                      <div className="form-group" style={{ marginBottom: 12 }}>
                        <label htmlFor="edit-workdir">Working directory</label>
                        <input id="edit-workdir" value={editWorkDir} onChange={e => setEditWorkDir(e.target.value)} placeholder="/home/deploy/my-app" />
                      </div>
                    )}
                    <div className="form-group" style={{ marginBottom: 12 }}>
                      <label htmlFor="edit-health">Health check path</label>
                      <input id="edit-health" value={editHealthPath} onChange={e => setEditHealthPath(e.target.value)} placeholder="/health" />
                      <p className="form-hint">Enables zero-downtime deploys. Your app must listen on $PORT.</p>
                    </div>
                  </div>
                  {configError && <p className="error-msg">{configError}</p>}
                  <div className="flex gap-sm mt-sm">
                    <button type="submit" className="btn btn-primary btn-sm">Save</button>
                    <button type="button" className="btn btn-secondary btn-sm" onClick={() => setEditing(false)}>Cancel</button>
                  </div>
                </form>
              ) : (
                <div className="config-grid">
                  <div className="config-item">
                    <span className="config-item-label">Start command</span>
                    <span className="config-item-value">{app.start_command}</span>
                  </div>
                  {app.build_command && (
                    <div className="config-item">
                      <span className="config-item-label">Build command</span>
                      <span className="config-item-value">{app.build_command}</span>
                    </div>
                  )}
                  <div className="config-item">
                    <span className="config-item-label">Source</span>
                    <span className="config-item-value">
                      {app.repo_url
                        ? `${app.repo_url} (${app.branch || 'main'})`
                        : app.work_dir || 'Local / uploaded'}
                    </span>
                  </div>
                  <div className="config-item">
                    <span className="config-item-label">Port</span>
                    <span className="config-item-value">{app.port}</span>
                  </div>
                  {app.health_path && (
                    <div className="config-item">
                      <span className="config-item-label">Health check</span>
                      <span className="config-item-value">{app.health_path} (zero-downtime deploys on)</span>
                    </div>
                  )}
                </div>
              )}
            </div>

            <div className="section">
              <h2>Environment Variables</h2>
              <form onSubmit={saveEnvs}>
                {envs.length === 0 && (
                  <p className="list-empty">No environment variables yet. They're injected into the process and written to a .env file on start.</p>
                )}
                {envs.map((env, i) => {
                  const visible = shownEnvValues.has(i);
                  return (
                    <div key={i} className="env-row">
                      <input placeholder="KEY" aria-label="Variable name" value={env.key} onChange={e => updateEnv(i, 'key', e.target.value)} />
                      <input placeholder="value" aria-label="Variable value" type={visible ? 'text' : 'password'} autoComplete="new-password" value={env.value} onChange={e => updateEnv(i, 'value', e.target.value)} />
                      <button type="button" className="btn btn-ghost btn-sm btn-icon" onClick={() => toggleEnvVisibility(i)} aria-label={visible ? 'Hide value' : 'Show value'} title={visible ? 'Hide value' : 'Show value'}>
                        {visible ? <EyeOffIcon /> : <EyeIcon />}
                      </button>
                      <button type="button" className="btn btn-ghost btn-sm" onClick={() => removeEnvRow(i)} style={{ flexShrink: 0 }}>Remove</button>
                    </div>
                  );
                })}
                {envError && <p className="error-msg">{envError}</p>}
                <div className="flex gap-sm mt-sm">
                  <button type="button" className="btn btn-ghost btn-sm" onClick={addEnvRow}>Add variable</button>
                  <button type="button" className="btn btn-ghost btn-sm" onClick={importEnvText}>Paste .env</button>
                  <button type="submit" className="btn btn-primary btn-sm">Save changes</button>
                </div>
              </form>
            </div>

            <div className="section">
              <h2>Domains</h2>
              {domains.length === 0 && (
                <p className="list-empty">
                  {system?.server_ip
                    ? <>No domains yet. Point an A record at <code>{system.server_ip}</code>, add the domain here, and SSL is automatic.</>
                    : 'No domains yet. Point a DNS A record at this server, add the domain, and SSL is automatic.'}
                </p>
              )}
              {domains.map(d => (
                <div key={d.id} className="domain-row">
                  <span>{d.domain}</span>
                  <button className="btn btn-ghost btn-sm" onClick={() => handleRemoveDomain(d.domain)}>Remove</button>
                </div>
              ))}
              {domainError && <p className="error-msg">{domainError}</p>}
              <form onSubmit={handleAddDomain} className="flex gap-sm mt-sm">
                <input placeholder="example.com" aria-label="Domain" value={newDomain} onChange={e => setNewDomain(e.target.value)} style={{ flex: 1 }} />
                <button type="submit" className="btn btn-primary btn-sm">Add domain</button>
              </form>
            </div>

            {app.webhook_secret && (
              <div className="section">
                <h2>Push to Deploy</h2>
                <p className="form-hint" style={{ marginBottom: 8 }}>
                  Add this URL as a webhook in your GitHub repo (Settings → Webhooks) or call it from CI.
                  Each delivery pulls, rebuilds, and restarts the app.
                </p>
                <div className="webhook-row">
                  <code className="webhook-url">{window.location.origin}/hooks/{app.id}?secret={'•'.repeat(12)}</code>
                  <button className="btn btn-secondary btn-sm" onClick={copyWebhookUrl}>
                    {copied ? 'Copied' : 'Copy URL'}
                  </button>
                </div>
                <p className="form-hint" style={{ marginTop: 8 }}>
                  GitHub webhooks can instead use the secret field with URL <code>{window.location.origin}/hooks/{app.id}</code> (content type: application/json).
                </p>
              </div>
            )}

            <div className="section">
              <h2>Danger Zone</h2>
              <div className="danger-zone">
                <div className="danger-zone-text">
                  <div className="danger-zone-title">Delete this app</div>
                  {app.work_dir
                    ? 'Stops the app and removes it from flightdeck. Your directory stays on the server.'
                    : 'Stops the app and removes it from flightdeck, including its app directory and logs.'}
                </div>
                <button className="btn btn-danger btn-sm" onClick={() => setConfirmingDelete(true)}>Delete app</button>
              </div>
            </div>
          </>
        )}
      </div>

      <ConfirmDialog
        open={rollbackTarget !== null}
        title={`Roll back to ${rollbackTarget?.commit_sha.slice(0, 7)}?`}
        message={`The working tree is reset to "${rollbackTarget?.commit_msg || rollbackTarget?.commit_sha.slice(0, 7)}" and the app restarts${app.health_path ? ' with zero downtime' : ''}. The next deploy or pull returns to the branch tip.`}
        confirmLabel="Roll back"
        onConfirm={handleRollback}
        onCancel={() => setRollbackTarget(null)}
      />

      <ConfirmDialog
        open={confirmingAction !== null}
        title={confirmingAction === 'stop' ? `Stop ${app.name}?` : `Restart ${app.name}?`}
        message={confirmingAction === 'stop'
          ? 'The app goes offline until you start it again. Anything it is serving right now is interrupted.'
          : 'The app restarts with its current configuration. There is a brief moment of downtime.'}
        confirmLabel={confirmingAction === 'stop' ? 'Stop app' : 'Restart app'}
        onConfirm={() => {
          const action = confirmingAction;
          setConfirmingAction(null);
          if (action === 'stop') handleAction('stop', () => stopApp(id!));
          if (action === 'restart') handleAction('restart', () => restartApp(id!));
        }}
        onCancel={() => setConfirmingAction(null)}
      />

      <ConfirmDialog
        open={confirmingDelete}
        title={`Delete ${app.name}?`}
        message={app.work_dir
          ? 'This stops the app and removes it from flightdeck. Your directory on the server is left untouched.'
          : 'This stops the app and removes it from flightdeck, including its app directory and logs.'}
        confirmLabel="Delete app"
        danger
        onConfirm={handleDelete}
        onCancel={() => setConfirmingDelete(false)}
      />
    </Layout>
  );
}

interface DeploymentRowProps {
  d: Deployment;
  isLive: boolean;
  expanded: boolean;
  expandedLines: string[];
  appName: string;
  canRollback: boolean;
  onToggleLog: (depId: string) => void;
  onRollback: (d: Deployment) => void;
}

function DeploymentRow({ d, isLive, expanded, expandedLines, appName, canRollback, onToggleLog, onRollback }: DeploymentRowProps) {
  const dur = duration(d.started_at, d.finished_at);
  return (
    <div>
      <div className="deployment-row">
        <span className={`badge badge-${d.status === 'success' ? 'running' : d.status === 'failed' ? 'crashed' : 'deploying'}`}>
          {d.status}
        </span>
        {isLive && <span className="badge badge-running" title="This deployment is currently serving">live</span>}
        <span className="deployment-trigger">{d.triggered_by}</span>
        {d.commit_sha && (
          <span className="deployment-commit" title={d.commit_sha}>
            {d.commit_sha.slice(0, 7)}{d.commit_msg ? ` ${d.commit_msg}` : ''}
          </span>
        )}
        <span className="deployment-time" title={exactTime(d.started_at)}>{relativeTime(d.started_at)}</span>
        {dur && <span className="deployment-time">{dur}</span>}
        <div className="deployment-rollback flex-center gap-xs">
          <button className="btn btn-ghost btn-sm" onClick={() => onToggleLog(d.id)}>
            {expanded ? 'Hide log' : 'View log'}
          </button>
          {canRollback && (
            <button className="btn btn-ghost btn-sm" onClick={() => onRollback(d)}>
              Roll back
            </button>
          )}
        </div>
      </div>
      {expanded && (
        <div className="mt-sm" style={{ marginBottom: 12 }}>
          <LogViewer
            lines={expandedLines}
            filename={`${appName}-deploy.log`}
            emptyText="No log was captured for this deployment (it predates deploy logging)."
            compact
          />
        </div>
      )}
    </div>
  );
}
