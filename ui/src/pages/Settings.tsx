import { useState, useEffect, type FormEvent } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  getSettings, updatePanelDomain, changePassword,
  updateGitToken, getSystemInfo, installRuntime,
  type Settings as SettingsType, type SystemInfo,
} from '../api';

export default function Settings() {
  const [settings, setSettings] = useState<SettingsType | null>(null);
  const [system, setSystem] = useState<SystemInfo | null>(null);
  const [domain, setDomain] = useState('');
  const [gitToken, setGitToken] = useState('');
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [msg, setMsg] = useState('');
  const [error, setError] = useState('');
  const [installing, setInstalling] = useState<string | null>(null);
  const location = useLocation();

  useEffect(() => { loadAll(); }, []);

  async function loadAll() {
    try {
      const [s, sys] = await Promise.all([getSettings(), getSystemInfo()]);
      setSettings(s);
      setSystem(sys);
      setDomain(s.panel_domain || '');
    } catch {}
  }

  function flash(message: string) {
    setError('');
    setMsg(message);
    setTimeout(() => setMsg(''), 3000);
  }

  async function handleDomain(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await updatePanelDomain(domain);
      flash('Domain updated');
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function handleGitToken(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await updateGitToken(gitToken);
      setGitToken('');
      await loadAll();
      flash('Git token updated');
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function handleRemoveToken() {
    setError('');
    try {
      await updateGitToken('');
      await loadAll();
      flash('Git token removed');
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function handleInstall(name: string) {
    setInstalling(name);
    setError('');
    try {
      await installRuntime(name);
      flash(`${name} installed successfully`);
      await loadAll();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setInstalling(null);
    }
  }

  async function handlePassword(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await changePassword(currentPw, newPw);
      setCurrentPw('');
      setNewPw('');
      flash('Password updated');
    } catch (err: any) {
      setError(err.message);
    }
  }

  return (
    <div className="layout">
      <nav className="nav">
        <div className="nav-left">
          <Link to="/" className="nav-brand">flightdeck</Link>
          <div className="nav-links">
            <Link to="/" className="nav-link">Apps</Link>
            <Link to="/settings" className={`nav-link ${location.pathname === '/settings' ? 'nav-link-active' : ''}`}>Settings</Link>
          </div>
        </div>
      </nav>
      <div className="container fade-in">
        <h1>Settings</h1>

        {msg && <p className="success-msg">{msg}</p>}
        {error && <p className="error-msg" style={{ marginBottom: 16 }}>{error}</p>}

        <div className="settings-grid">
          {system && (
            <div className="card">
              <h2>System</h2>
              <div className="runtime-grid">
                <div className={`runtime-card ${system.caddy.running ? '' : 'runtime-card-missing'}`}>
                  <div className="runtime-card-header">
                    <span className={`runtime-dot ${system.caddy.running ? 'runtime-dot-ok' : 'runtime-dot-fail'}`} />
                    <span className="runtime-card-name">Caddy</span>
                  </div>
                  <span className="runtime-card-version">
                    {system.caddy.running ? (system.caddy.version || 'running') : 'Not running'}
                  </span>
                </div>
                {system.runtimes.map(r => (
                  <div key={r.name} className={`runtime-card ${r.installed ? '' : 'runtime-card-installable'}`}>
                    <div className="runtime-card-header">
                      <span className={`runtime-dot ${r.installed ? 'runtime-dot-ok' : 'runtime-dot-fail'}`} />
                      <span className="runtime-card-name">{r.name}</span>
                    </div>
                    <span className="runtime-card-version">
                      {r.installed ? r.version : 'Not installed'}
                    </span>
                    {!r.installed && r.name !== 'Git' && (
                      <button
                        className="btn btn-primary btn-sm"
                        onClick={() => handleInstall(r.name)}
                        disabled={installing !== null}
                        style={{ width: '100%', marginTop: 8 }}
                      >
                        {installing === r.name ? <><span className="spinner" /> Installing...</> : 'Install'}
                      </button>
                    )}
                  </div>
                ))}
              </div>
              <p className="form-hint mt-sm">
                {system.os}/{system.arch}
              </p>
            </div>
          )}

          <div className="card">
            <h2>Git Authentication</h2>
            {settings?.has_git_token ? (
              <div>
                <div className="token-display">
                  <span>ghp_••••••••••••••••••••</span>
                </div>
                <div className="flex gap-sm mt-sm">
                  <button className="btn btn-ghost btn-sm" onClick={() => setGitToken('')}>Replace</button>
                  <button className="btn btn-danger btn-sm" onClick={handleRemoveToken}>Remove token</button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleGitToken}>
                <div className="form-group">
                  <label>Personal access token</label>
                  <input
                    type="password"
                    value={gitToken}
                    onChange={e => setGitToken(e.target.value)}
                    placeholder="ghp_xxxxxxxxxxxxxxxxxxxx"
                  />
                  <p className="form-hint">Required for private repositories</p>
                </div>
                <button type="submit" className="btn btn-primary btn-sm" disabled={!gitToken}>Save token</button>
              </form>
            )}
          </div>

          <div className="card">
            <h2>Control Panel Domain</h2>
            <form onSubmit={handleDomain}>
              <div className="form-group">
                <label>Domain</label>
                <input value={domain} onChange={e => setDomain(e.target.value)} placeholder="admin.example.com" />
                <p className="form-hint">Leave blank for IP-only access on :3000</p>
              </div>
              <button type="submit" className="btn btn-primary btn-sm">Update domain</button>
            </form>
          </div>

          <div className="card">
            <h2>Change Password</h2>
            <form onSubmit={handlePassword}>
              <div className="form-group">
                <label>Current password</label>
                <input type="password" value={currentPw} onChange={e => setCurrentPw(e.target.value)} />
              </div>
              <div className="form-group">
                <label>New password</label>
                <input type="password" value={newPw} onChange={e => setNewPw(e.target.value)} placeholder="Minimum 8 characters" />
              </div>
              <button type="submit" className="btn btn-primary btn-sm">Update password</button>
            </form>
          </div>
        </div>
      </div>
    </div>
  );
}
