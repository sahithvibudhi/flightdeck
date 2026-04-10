import { useState, useEffect, useRef, type FormEvent } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import {
  getApp, deleteApp, startApp, stopApp, restartApp, pullApp, updateApp,
  getAppLogs, listEnvs, replaceEnvs, listDomains, addDomain, removeDomain,
  type App, type EnvVar, type DomainEntry,
} from '../api';

function formatMemory(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  if (mb > 0) return `${mb.toFixed(1)} MB`;
  return '—';
}

 

function EyeIcon(){
  return(
    <svg width = "14" height ="14" viewBox = "0 0 24 24" fill = "none" stroke ="currentColor" strokeWidth = "1.5">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
      <circle cx="12" cy ="12" r="3" />
    </svg>
  );
  }

function EyeOffIcon(){
  return (
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" 
  strokeWidth="1.5">                                                                                 
        <path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-11-8-11-8a18.45 18.45 0 015.06-5.94" />
        <path d="M9.9 4.24A9.12 9.12 0 0112 4c7 0 11 8 11 8a18.5 18.5 0 01-2.16 3.19" />             
        <line x1="1" y1="1" x2="23" y2="23" />                                                       
      </svg>                                                                                         
    ); 
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
  const [shownEnvValues, setShownEnvValues] = useState<Set<number>>(new Set());

  useEffect(() => {
    if (!id) return;
    loadAll();
    const logInterval = setInterval(loadLogs, 3000);
    const metricInterval = setInterval(loadApp, 5000);
    return () => {
      clearInterval(logInterval);
      clearInterval(metricInterval);
    };
  }, [id]);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs]);

  async function loadAll() {
    await Promise.all([loadApp(), loadLogs(), loadEnvs(), loadDomains()]);
  }

  async function loadApp() {
    try { setApp(await getApp(id!)); } catch {}
  }

  async function loadLogs() {
    try {
      const res = await getAppLogs(id!);
      setLogs(res.lines);
    } catch {}
  }

  async function loadEnvs() {
    try { setEnvs(await listEnvs(id!)); } catch {}
  }

  async function loadDomains() {
    try { setDomains(await listDomains(id!)); } catch {}
  }

  async function handleAction(label: string, action: () => Promise<any>) {
    setError('');
    setActionLoading(label);
    try {
      await action();
      await loadApp();
    } catch (err: any) {
      setError(err.message);
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
    } catch (err: any) {
      setError(err.message);
    } finally {
      setPulling(false);
    }
  }

  async function handleDelete() {
    if (!confirm(`Delete ${app?.name}? This will stop the app and remove all data.`)) return;
    try {
      await deleteApp(id!);
      navigate('/');
    } catch (err: any) {
      setError(err.message);
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

  function toggleEnvVisibility(index: number){
    setShownEnvValues(prev =>{
      const next = new Set(prev);
      next.has(index) ? next.delete(index) : next.add(index);
      return next;
    });
  }

  async function saveEnvs(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await replaceEnvs(id!, envs.filter(e => e.key.trim() !== ''));
      await loadEnvs();
    } catch (err: any) {
      setError(err.message);
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
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function handleRemoveDomain(domain: string) {
    setError('');
    try {
      await removeDomain(id!, domain);
      await loadDomains();
    } catch (err: any) {
      setError(err.message);
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
      });
      setEditing(false);
      await loadApp();
    } catch (err: any) {
      setError(err.message);
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
            </div>
          </div>
          <div className="app-actions">
            {app.repo_url && (
              <button
                className="btn btn-secondary btn-sm"
                onClick={handlePull}
                disabled={pulling}
              >
                {pulling ? <><span className="spinner" /> Pulling...</> : 'Pull'}
              </button>
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
            <button className="btn btn-danger btn-sm" onClick={handleDelete}>Delete</button>
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
                  <label>App name</label>
                  <input value={editName} onChange={e => setEditName(e.target.value)} />
                </div>
                <div className="form-group" style={{ marginBottom: 12 }}>
                  <label>Start command</label>
                  <input value={editStartCmd} onChange={e => setEditStartCmd(e.target.value)} />
                </div>
                <div className="form-group" style={{ marginBottom: 12 }}>
                  <label>Build command</label>
                  <input value={editBuildCmd} onChange={e => setEditBuildCmd(e.target.value)} placeholder="e.g. npm install" />
                </div>
                <div className="form-group" style={{ marginBottom: 12 }}>
                  <label>Port</label>
                  <input value={editPort} onChange={e => setEditPort(e.target.value.replace(/\D/g, ''))} placeholder="e.g. 3000" />
                </div>
                <div className="form-group" style={{ marginBottom: 12 }}>
                  <label>Repository URL</label>
                  <input value={editRepoUrl} onChange={e => setEditRepoUrl(e.target.value)} placeholder="https://github.com/..." />
                </div>
                <div className="form-group" style={{ marginBottom: 12 }}>
                  <label>Branch</label>
                  <input value={editBranch} onChange={e => setEditBranch(e.target.value)} placeholder="main" />
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
                  {app.repo_url ? `${app.repo_url} (${app.branch || 'main'})` : 'Local / uploaded'}
                </span>
              </div>
              <div className="config-item">
                <span className="config-item-label">Port</span>
                <span className="config-item-value">{app.port}</span>
              </div>
            </div>
          )}
        </div>

        <div className="section">
          <h2>Logs</h2>
          <div className="log-output" ref={logRef}>
            {logs.length > 0 ? logs.join('\n') : 'Waiting for output...'}
          </div>
        </div>

        <div className="section">
          <h2>Environment Variables</h2>
          <form onSubmit={saveEnvs}>
            {envs.map((env, i) => {
              const visible = shownEnvValues.has(i);                                                                                                
              return (                                            
              <div key={i} className="env-row">                                                                                                   
                <input placeholder="KEY" value={env.key} onChange={e => updateEnv(i, 'key', e.target.value)} />
                <input placeholder="value" type={visible ? 'text' : 'password'} autoComplete="new-password" value={env.value} onChange={e => updateEnv(i, 'value', e.target.value)} />                                                                                                                                
                <button type="button" className="btn btn-ghost btn-sm btn-icon" onClick={() => toggleEnvVisibility(i)} title={visible ? 'Hide value' : 'Show value'} >  {visible ? <EyeOffIcon /> : <EyeIcon />}  </button>                                     
                                                                                                                                                                                                                                           
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
          {domains.map(d => (
            <div key={d.id} className="domain-row">
              <span>{d.domain}</span>
              <button className="btn btn-ghost btn-sm" onClick={() => handleRemoveDomain(d.domain)}>Remove</button>
            </div>
          ))}
          <form onSubmit={handleAddDomain} className="flex gap-sm mt-sm">
            <input placeholder="example.com" value={newDomain} onChange={e => setNewDomain(e.target.value)} style={{ flex: 1 }} />
            <button type="submit" className="btn btn-primary btn-sm">Add domain</button>
          </form>
        </div>
      </div>
    </div>
  );} 
