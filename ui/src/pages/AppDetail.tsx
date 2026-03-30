import { useState, useEffect, FormEvent } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import {
  getApp, deleteApp, startApp, stopApp, restartApp,
  getAppLogs, listEnvs, replaceEnvs, listDomains, addDomain, removeDomain,
  App, EnvVar, DomainEntry,
} from '../api';

export default function AppDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [app, setApp] = useState<App | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [envs, setEnvs] = useState<EnvVar[]>([]);
  const [domains, setDomains] = useState<DomainEntry[]>([]);
  const [newDomain, setNewDomain] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    if (!id) return;
    loadAll();
    const interval = setInterval(loadLogs, 3000);
    return () => clearInterval(interval);
  }, [id]);

  async function loadAll() {
    await Promise.all([loadApp(), loadLogs(), loadEnvs(), loadDomains()]);
  }

  async function loadApp() {
    try {
      setApp(await getApp(id!));
    } catch {}
  }

  async function loadLogs() {
    try {
      const res = await getAppLogs(id!);
      setLogs(res.lines);
    } catch {}
  }

  async function loadEnvs() {
    try {
      setEnvs(await listEnvs(id!));
    } catch {}
  }

  async function loadDomains() {
    try {
      setDomains(await listDomains(id!));
    } catch {}
  }

  async function handleAction(action: () => Promise<any>) {
    setError('');
    try {
      await action();
      await loadApp();
    } catch (err: any) {
      setError(err.message);
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

  async function saveEnvs(e: FormEvent) {
    e.preventDefault();
    const filtered = envs.filter(e => e.key.trim() !== '');
    try {
      await replaceEnvs(id!, filtered);
      await loadEnvs();
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function handleAddDomain(e: FormEvent) {
    e.preventDefault();
    if (!newDomain) return;
    try {
      await addDomain(id!, newDomain);
      setNewDomain('');
      await loadDomains();
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function handleRemoveDomain(domain: string) {
    try {
      await removeDomain(id!, domain);
      await loadDomains();
    } catch (err: any) {
      setError(err.message);
    }
  }

  if (!app) return <div className="container">Loading...</div>;

  return (
    <>
      <nav className="nav">
        <Link to="/" className="nav-brand">nestops</Link>
        <div className="nav-links">
          <Link to="/settings">Settings</Link>
        </div>
      </nav>
      <div className="container">
        <div className="flex-between">
          <div>
            <h1>{app.name}</h1>
            <span className={`badge badge-${app.status}`}>{app.status}</span>
            <span className="app-port" style={{ marginLeft: '0.5rem' }}>port {app.port}</span>
          </div>
          <div className="flex gap-sm">
            {app.status !== 'running' && (
              <button className="btn btn-primary btn-sm" onClick={() => handleAction(() => startApp(id!))}>Start</button>
            )}
            {app.status === 'running' && (
              <button className="btn btn-ghost btn-sm" onClick={() => handleAction(() => stopApp(id!))}>Stop</button>
            )}
            <button className="btn btn-ghost btn-sm" onClick={() => handleAction(() => restartApp(id!))}>Restart</button>
            <button className="btn btn-danger btn-sm" onClick={handleDelete}>Delete</button>
          </div>
        </div>

        {error && <p className="error-msg">{error}</p>}

        {/* Logs */}
        <div className="section mt-md">
          <h2>Logs</h2>
          <div className="log-output">
            {logs.length > 0 ? logs.join('\n') : 'No logs yet.'}
          </div>
        </div>

        {/* Env Vars */}
        <div className="section">
          <h2>Environment Variables</h2>
          <form onSubmit={saveEnvs}>
            {envs.map((env, i) => (
              <div key={i} className="env-row">
                <input placeholder="KEY" value={env.key} onChange={e => updateEnv(i, 'key', e.target.value)} />
                <input placeholder="value" value={env.value} onChange={e => updateEnv(i, 'value', e.target.value)} />
                <button type="button" className="btn btn-ghost btn-sm" onClick={() => removeEnvRow(i)}>×</button>
              </div>
            ))}
            <div className="flex gap-sm mt-md">
              <button type="button" className="btn btn-ghost btn-sm" onClick={addEnvRow}>Add variable</button>
              <button type="submit" className="btn btn-primary btn-sm">Save</button>
            </div>
          </form>
        </div>

        {/* Domains */}
        <div className="section">
          <h2>Domains</h2>
          {domains.map(d => (
            <div key={d.id} className="flex-between" style={{ marginBottom: '0.5rem' }}>
              <span>{d.domain}</span>
              <button className="btn btn-ghost btn-sm" onClick={() => handleRemoveDomain(d.domain)}>Remove</button>
            </div>
          ))}
          <form onSubmit={handleAddDomain} className="flex gap-sm mt-md">
            <input
              placeholder="example.com"
              value={newDomain}
              onChange={e => setNewDomain(e.target.value)}
              style={{ flex: 1 }}
            />
            <button type="submit" className="btn btn-primary btn-sm">Add domain</button>
          </form>
        </div>
      </div>
    </>
  );
}
