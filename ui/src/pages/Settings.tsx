import { useState, useEffect, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import {
  getSettings, updatePanelDomain, changePassword,
  updateGitToken, getSystemInfo,
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

  useEffect(() => {
    loadAll();
  }, []);

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
        <Link to="/" className="nav-brand">nestops</Link>
        <div className="nav-links">
          <Link to="/">Apps</Link>
        </div>
      </nav>
      <div className="container fade-in">
        <h1>Settings</h1>

        {msg && <p className="success-msg">{msg}</p>}
        {error && <p className="error-msg" style={{ marginBottom: 16 }}>{error}</p>}

        <div className="settings-grid">
          <div className="card">
            <h2>System</h2>
            {system && (
              <div className="system-check">
                {system.git.installed ? (
                  <span className="system-check-ok">
                    Git {system.git.version}
                  </span>
                ) : (
                  <span className="system-check-fail">
                    Git is not installed
                  </span>
                )}
              </div>
            )}
          </div>

          <div className="card">
            <h2>Git Authentication</h2>
            {settings?.has_git_token ? (
              <div>
                <div className="token-display">
                  <span>ghp_••••••••••••••••••••</span>
                </div>
                <div className="flex gap-sm mt-sm">
                  <button className="btn btn-ghost btn-sm" onClick={() => setGitToken('')}>
                    Replace
                  </button>
                  <button className="btn btn-danger btn-sm" onClick={handleRemoveToken}>
                    Remove token
                  </button>
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
                  <p className="form-hint">
                    Required for private repositories. Create one at GitHub → Settings → Developer settings → Personal access tokens.
                  </p>
                </div>
                <button type="submit" className="btn btn-primary btn-sm" disabled={!gitToken}>
                  Save token
                </button>
              </form>
            )}
          </div>

          <div className="card">
            <h2>Control Panel Domain</h2>
            <form onSubmit={handleDomain}>
              <div className="form-group">
                <label>Domain</label>
                <input
                  value={domain}
                  onChange={e => setDomain(e.target.value)}
                  placeholder="admin.example.com"
                />
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
                <input
                  type="password"
                  value={currentPw}
                  onChange={e => setCurrentPw(e.target.value)}
                />
              </div>
              <div className="form-group">
                <label>New password</label>
                <input
                  type="password"
                  value={newPw}
                  onChange={e => setNewPw(e.target.value)}
                  placeholder="Minimum 8 characters"
                />
              </div>
              <button type="submit" className="btn btn-primary btn-sm">Update password</button>
            </form>
          </div>
        </div>
      </div>
    </div>
  );
}
