import { useState, useEffect, type FormEvent } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  getSettings, updatePanelDomain, changePassword,
  updateGitToken, getSystemInfo, installRuntime,
  updateNotifications, testNotifications,
  listApiTokens, createApiToken, deleteApiToken,
  errMsg,
  type Settings as SettingsType, type SystemInfo, type ApiToken,
} from '../api';
import { toast } from '../components/toastBus';
import ConfirmDialog from '../components/ConfirmDialog';

export default function Settings() {
  const [settings, setSettings] = useState<SettingsType | null>(null);
  const [system, setSystem] = useState<SystemInfo | null>(null);
  const [domain, setDomain] = useState('');
  const [gitToken, setGitToken] = useState('');
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [error, setError] = useState('');
  const [installing, setInstalling] = useState<string | null>(null);
  const [replacingToken, setReplacingToken] = useState(false);
  const [notifyDiscord, setNotifyDiscord] = useState('');
  const [notifyTgToken, setNotifyTgToken] = useState('');
  const [notifyTgChat, setNotifyTgChat] = useState('');
  const [notifyWebhook, setNotifyWebhook] = useState('');
  const [testingNotify, setTestingNotify] = useState(false);
  const [apiTokens, setApiTokens] = useState<ApiToken[]>([]);
  const [tokenName, setTokenName] = useState('');
  const [tokenScope, setTokenScope] = useState('read');
  const [newToken, setNewToken] = useState('');
  const [tokenCopied, setTokenCopied] = useState(false);
  const [deletingToken, setDeletingToken] = useState<ApiToken | null>(null);
  const location = useLocation();

  useEffect(() => { loadAll(); }, []);

  async function loadAll() {
    try {
      const [s, sys, tokens] = await Promise.all([getSettings(), getSystemInfo(), listApiTokens()]);
      setSettings(s);
      setSystem(sys);
      setApiTokens(tokens);
      setDomain(s.panel_domain || '');
      setNotifyDiscord(s.notify_discord || '');
      setNotifyTgToken(s.notify_telegram_token || '');
      setNotifyTgChat(s.notify_telegram_chat || '');
      setNotifyWebhook(s.notify_webhook || '');
    } catch { /* transient */ }
  }

  function flash(message: string) {
    setError('');
    toast(message);
  }

  async function handleDomain(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await updatePanelDomain(domain.trim().toLowerCase());
      flash('Domain updated');
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function handleGitToken(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await updateGitToken(gitToken);
      setGitToken('');
      setReplacingToken(false);
      await loadAll();
      flash('Git token updated');
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function handleRemoveToken() {
    setError('');
    try {
      await updateGitToken('');
      await loadAll();
      flash('Git token removed');
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function handleInstall(name: string) {
    setInstalling(name);
    setError('');
    try {
      await installRuntime(name);
      flash(`${name} installed successfully`);
      await loadAll();
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setInstalling(null);
    }
  }

  async function handleNotifications(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await updateNotifications({
        discord: notifyDiscord,
        telegram_token: notifyTgToken,
        telegram_chat: notifyTgChat,
        webhook: notifyWebhook,
      });
      flash('Notification settings saved');
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function handleCreateToken(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      const created = await createApiToken(tokenName.trim(), tokenScope);
      setNewToken(created.token);
      setTokenCopied(false);
      setTokenName('');
      setApiTokens(await listApiTokens());
      flash(`Token "${created.name}" created`);
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function handleTestNotifications() {
    setError('');
    setTestingNotify(true);
    try {
      await testNotifications();
      flash('Test notification sent');
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setTestingNotify(false);
    }
  }

  async function handleDeleteToken() {
    if (!deletingToken) return;
    const target = deletingToken;
    setDeletingToken(null);
    setError('');
    try {
      await deleteApiToken(target.id);
      setApiTokens(await listApiTokens());
      flash(`Token "${target.name}" deleted`);
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function copyNewToken() {
    try {
      await navigator.clipboard.writeText(newToken);
      setTokenCopied(true);
      setTimeout(() => setTokenCopied(false), 2000);
    } catch {
      setError('Could not copy the token, copy it manually');
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
    } catch (err) {
      setError(errMsg(err));
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
                    {system.caddy.running ? (system.caddy.version || 'running') : 'Not running — domains and SSL disabled'}
                  </span>
                  {!system.caddy.running && (
                    <button
                      className="btn btn-primary btn-sm"
                      onClick={() => handleInstall('caddy')}
                      disabled={installing !== null}
                      style={{ width: '100%', marginTop: 8 }}
                    >
                      {installing === 'caddy' ? <><span className="spinner" /> Installing...</> : 'Install & start'}
                    </button>
                  )}
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
                    {!r.installed && (
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
            {settings?.has_git_token && !replacingToken ? (
              <div>
                <div className="token-display">
                  <span>••••••••••••••••••••</span>
                </div>
                <div className="flex gap-sm mt-sm">
                  <button className="btn btn-ghost btn-sm" onClick={() => { setGitToken(''); setReplacingToken(true); }}>Replace</button>
                  <button className="btn btn-danger btn-sm" onClick={handleRemoveToken}>Remove token</button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleGitToken}>
                <div className="form-group">
                  <label htmlFor="git-token">Personal access token</label>
                  <input
                    id="git-token"
                    type="password"
                    value={gitToken}
                    onChange={e => setGitToken(e.target.value)}
                    placeholder="ghp_xxxxxxxxxxxxxxxxxxxx"
                  />
                  <p className="form-hint">Required for private repositories</p>
                </div>
                <div className="flex gap-sm">
                  <button type="submit" className="btn btn-primary btn-sm" disabled={!gitToken}>Save token</button>
                  {replacingToken && (
                    <button type="button" className="btn btn-ghost btn-sm" onClick={() => setReplacingToken(false)}>Cancel</button>
                  )}
                </div>
              </form>
            )}
          </div>

          <div className="card">
            <h2>Control Panel Domain</h2>
            <form onSubmit={handleDomain}>
              <div className="form-group">
                <label htmlFor="panel-domain">Domain</label>
                <input id="panel-domain" value={domain} onChange={e => setDomain(e.target.value)} placeholder="admin.example.com" />
                <p className="form-hint">Leave blank for IP-only access on :3000</p>
              </div>
              <button type="submit" className="btn btn-primary btn-sm">Update domain</button>
            </form>
          </div>

          <div className="card">
            <h2>Notifications</h2>
            <p className="form-hint" style={{ marginBottom: 12 }}>
              Sent on deploy success, deploy failure, and app crashes. Leave a field blank to disable that channel.
            </p>
            <form onSubmit={handleNotifications}>
              <div className="form-group">
                <label htmlFor="notify-discord">Discord webhook URL</label>
                <input id="notify-discord" value={notifyDiscord} onChange={e => setNotifyDiscord(e.target.value)} placeholder="https://discord.com/api/webhooks/..." />
              </div>
              <div className="form-group">
                <label htmlFor="notify-tg-token">Telegram bot token</label>
                <input id="notify-tg-token" value={notifyTgToken} onChange={e => setNotifyTgToken(e.target.value)} placeholder="123456:ABC..." />
              </div>
              <div className="form-group">
                <label htmlFor="notify-tg-chat">Telegram chat ID</label>
                <input id="notify-tg-chat" value={notifyTgChat} onChange={e => setNotifyTgChat(e.target.value)} placeholder="-100123456789" />
              </div>
              <div className="form-group">
                <label htmlFor="notify-webhook">Generic webhook URL</label>
                <input id="notify-webhook" value={notifyWebhook} onChange={e => setNotifyWebhook(e.target.value)} placeholder="https://example.com/hook" />
                <p className="form-hint">Receives JSON: title, message, timestamp</p>
              </div>
              <div className="flex gap-sm">
                <button type="submit" className="btn btn-primary btn-sm">Save</button>
                <button type="button" className="btn btn-secondary btn-sm" onClick={handleTestNotifications} disabled={testingNotify}>
                  {testingNotify ? <><span className="spinner" /> Sending...</> : 'Send test'}
                </button>
              </div>
            </form>
          </div>

          <div className="card">
            <h2>API Tokens</h2>
            <p className="form-hint" style={{ marginBottom: 12 }}>
              Revocable tokens for CLI and CI use. <code>read</code> allows GET requests;
              <code> deploy</code> also allows start, stop, restart, pull, and deploy.
            </p>

            {newToken && (
              <div style={{ marginBottom: 16 }}>
                <div className="webhook-row">
                  <code className="webhook-url">{newToken}</code>
                  <button className="btn btn-secondary btn-sm" onClick={copyNewToken}>
                    {tokenCopied ? 'Copied' : 'Copy'}
                  </button>
                </div>
                <p className="form-hint" style={{ marginTop: 8 }}>
                  Copy it now, it is not shown again.
                </p>
              </div>
            )}

            {apiTokens.length === 0 ? (
              <p className="list-empty">No tokens yet</p>
            ) : (
              apiTokens.map(t => (
                <div key={t.id} className="webhook-row" style={{ marginBottom: 8 }}>
                  <div style={{ flex: 1 }}>
                    <div>{t.name} <span className="form-hint" style={{ display: 'inline' }}>({t.scope})</span></div>
                    <p className="form-hint">
                      Created {t.created_at} · {t.last_used ? `Last used ${t.last_used}` : 'Never used'}
                    </p>
                  </div>
                  <button className="btn btn-danger btn-sm" onClick={() => setDeletingToken(t)}>Delete</button>
                </div>
              ))
            )}

            <form onSubmit={handleCreateToken} className="flex gap-sm mt-sm" style={{ alignItems: 'flex-end' }}>
              <div className="form-group" style={{ flex: 1, marginBottom: 0 }}>
                <label htmlFor="token-name">Name</label>
                <input
                  id="token-name"
                  value={tokenName}
                  onChange={e => setTokenName(e.target.value)}
                  placeholder="deploy-bot"
                  maxLength={60}
                />
              </div>
              <div className="form-group" style={{ marginBottom: 0 }}>
                <label htmlFor="token-scope">Scope</label>
                <select id="token-scope" value={tokenScope} onChange={e => setTokenScope(e.target.value)}>
                  <option value="read">read</option>
                  <option value="deploy">deploy</option>
                </select>
              </div>
              <button type="submit" className="btn btn-primary btn-sm" disabled={!tokenName.trim()}>Create token</button>
            </form>
          </div>

          <div className="card">
            <h2>Change Password</h2>
            <form onSubmit={handlePassword}>
              <div className="form-group">
                <label htmlFor="current-pw">Current password</label>
                <input id="current-pw" type="password" value={currentPw} onChange={e => setCurrentPw(e.target.value)} />
              </div>
              <div className="form-group">
                <label htmlFor="new-pw">New password</label>
                <input id="new-pw" type="password" value={newPw} onChange={e => setNewPw(e.target.value)} placeholder="Minimum 8 characters" />
              </div>
              <button type="submit" className="btn btn-primary btn-sm">Update password</button>
            </form>
          </div>
        </div>
      </div>

      <ConfirmDialog
        open={deletingToken !== null}
        title="Delete API token"
        message={`Delete token "${deletingToken?.name}"? Anything still using it will stop working immediately.`}
        confirmLabel="Delete"
        danger
        onConfirm={handleDeleteToken}
        onCancel={() => setDeletingToken(null)}
      />
    </div>
  );
}
