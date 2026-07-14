import { useState, useEffect, useRef, useCallback, type FormEvent } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import {
  getApp, deleteApp, startApp, stopApp, restartApp, pullApp, updateApp,
  getAppLogs, streamAppLogs, listEnvs, replaceEnvs, listDomains, addDomain, removeDomain,
  listDeployments, deployApp, getSystemInfo,
  errMsg,
  type App, type EnvVar, type DomainEntry, type Deployment, type SystemInfo,
} from '../api';
import { EyeIcon, EyeOffIcon, ExternalLinkIcon } from '../components/Icons';
import { toast } from '../components/toastBus';
import ConfirmDialog from '../components/ConfirmDialog';

function formatMemory(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  if (mb > 0) return `${mb.toFixed(1)} MB`;
  return '—';
}

export default function AppDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const logRef = useRef<HTMLDivElement>(null);
  const [app, setApp] = useState<App | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [envs, setEnvs] = useState<EnvVar[]>([]);
  const [domains, setDomains] = useState<DomainEntry[]>([]);
  const [newDomain, setNewDomain] = useState('');
  const [error, setError] = useState('');
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
  const [copied, setCopied] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [system, setSystem] = useState<SystemInfo | null>(null);

  useEffect(() => {
    getSystemInfo().then(setSystem).catch(() => { /* transient */ });
  }, []);

  const loadApp = useCallback(async () => {
    try { setApp(await getApp(id!)); } catch { /* transient */ }
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

  // Logs stream live over SSE; if the stream can't connect (proxy in the
  // way, old browser) fall back to 3s polling.
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

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs]);

  async function handleAction(label: string, action: () => Promise<unknown>) {
    setError('');
    setActionLoading(label);
    try {
      await action();
      await loadApp();
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setActionLoading('');
    }
  }

  async function handlePull() {
    setError('');
    setPullOutput('');
    setPulling(true);
    try {
      const res = await pullApp(id!);
      setPullOutput(res.output);
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setPulling(false);
    }
  }

  async function handleDeploy() {
    setError('');
    try {
      await deployApp(id!);
      toast('Deploy started');
      await loadDeployments();
    } catch (err) {
      setError(errMsg(err));
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
      setError('Could not copy — copy the URL manually');
    }
  }

  async function handleDelete() {
    setConfirmingDelete(false);
    try {
      await deleteApp(id!);
      navigate('/');
    } catch (err) {
      setError(errMsg(err));
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
    setError('');
    try {
      await replaceEnvs(id!, envs.filter(e => e.key.trim() !== ''));
      await loadEnvs();
      toast(app?.status === 'running'
        ? 'Environment variables saved — restart to apply'
        : 'Environment variables saved');
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function handleAddDomain(e: FormEvent) {
    e.preventDefault();
    if (!newDomain) return;
    setError('');
    try {
      await addDomain(id!, newDomain);
      setNewDomain('');
      await loadDomains();
      toast('Domain added — SSL certificate provisions automatically');
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function handleRemoveDomain(domain: string) {
    setError('');
    try {
      await removeDomain(id!, domain);
      await loadDomains();
      toast('Domain removed');
    } catch (err) {
      setError(errMsg(err));
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
    setEditing(true);
  }

  async function handleSaveConfig(e: FormEvent) {
    e.preventDefault();
    setError('');
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
      setError(errMsg(err));
    }
  }

  if (!app) {
    return (
      <div className="layout">
        <nav className="nav">
          <div className="nav-left">
            <Link to="/" className="nav-brand">flightdeck</Link>
          </div>
        </nav>
        <div className="container">
          <div className="flex-center gap-sm" style={{ padding: '40px 0', justifyContent: 'center' }}>
            <span className="spinner" />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="layout">
      <nav className="nav">
        <div className="nav-left">
          <Link to="/" className="nav-brand">flightdeck</Link>
          <div className="nav-links">
            <Link to="/" className="nav-link nav-link-active">Apps</Link>
            <Link to="/settings" className="nav-link">Settings</Link>
          </div>
        </div>
      </nav>
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
                  href={domains.length > 0 ? `https://${domains[0].domain}` : `http://${system!.server_ip}:${app.port}`}
                  target="_blank"
                  rel="noreferrer"
                >
                  {domains.length > 0 ? domains[0].domain : `${system!.server_ip}:${app.port}`}
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
                  disabled={!!actionLoading}
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
                onClick={() => handleAction('stop', () => stopApp(id!))}
                disabled={!!actionLoading}
              >
                {actionLoading === 'stop' ? <span className="spinner" /> : 'Stop'}
              </button>
            )}
            <button
              className="btn btn-secondary btn-sm"
              onClick={() => handleAction('restart', () => restartApp(id!))}
              disabled={!!actionLoading}
            >
              {actionLoading === 'restart' ? <span className="spinner" /> : 'Restart'}
            </button>
            <button className="btn btn-danger btn-sm" onClick={() => setConfirmingDelete(true)}>Delete</button>
          </div>
        </div>

        {error && <p className="error-msg">{error}</p>}

        {pullOutput && (
          <div className="pull-output fade-in">{pullOutput}</div>
        )}

        {app.status === 'running' && (
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
              <span className="metrics-bar-value">:{app.port}</span>
              <span className="metrics-bar-label">Port</span>
            </div>
          </div>
        )}

        <div className="section mt-md">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
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

        {deployments.length > 0 && (
          <div className="section">
            <h2>Deployments</h2>
            {deployments.map(d => (
              <div key={d.id} className="deployment-row">
                <span className={`badge badge-${d.status === 'success' ? 'running' : d.status === 'failed' ? 'crashed' : 'deploying'}`}>
                  {d.status}
                </span>
                <span className="deployment-trigger">{d.triggered_by}</span>
                {d.commit_sha && (
                  <span className="deployment-commit" title={d.commit_sha}>
                    {d.commit_sha.slice(0, 7)}{d.commit_msg ? ` ${d.commit_msg}` : ''}
                  </span>
                )}
                <span className="deployment-time">{d.started_at}</span>
                {d.detail && <span className="deployment-detail" title={d.detail}>{d.detail.split('\n')[0]}</span>}
              </div>
            ))}
          </div>
        )}

        <div className="section">
          <h2>Logs</h2>
          <div className="log-output" ref={logRef}>
            {logs.length > 0 ? logs.join('\n') : 'Waiting for output...'}
          </div>
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
            <div className="flex gap-sm mt-sm">
              <button type="button" className="btn btn-ghost btn-sm" onClick={addEnvRow}>Add variable</button>
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
          <form onSubmit={handleAddDomain} className="flex gap-sm mt-sm">
            <input placeholder="example.com" aria-label="Domain" value={newDomain} onChange={e => setNewDomain(e.target.value)} style={{ flex: 1 }} />
            <button type="submit" className="btn btn-primary btn-sm">Add domain</button>
          </form>
        </div>
      </div>

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
    </div>
  );
}
