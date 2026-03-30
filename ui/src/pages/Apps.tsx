import { useState, useEffect, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { listApps, createApp, clearToken, type App } from '../api';

type DeployMode = 'github' | 'manual';

function repoShortName(url: string): string {
  return url.replace(/^https?:\/\/(www\.)?github\.com\//, '').replace(/\.git$/, '');
}

export default function Apps() {
  const [apps, setApps] = useState<App[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [mode, setMode] = useState<DeployMode>('github');
  const [name, setName] = useState('');
  const [startCmd, setStartCmd] = useState('');
  const [repoUrl, setRepoUrl] = useState('');
  const [branch, setBranch] = useState('main');
  const [error, setError] = useState('');
  const [deploying, setDeploying] = useState(false);
  const navigate = useNavigate();

  useEffect(() => { loadApps(); }, []);

  async function loadApps() {
    try { setApps(await listApps()); } catch {}
  }

  function inferName(url: string) {
    const parts = url.replace(/\.git$/, '').split('/');
    return parts[parts.length - 1] || '';
  }

  function handleRepoUrlChange(url: string) {
    setRepoUrl(url);
    if (!name || name === inferName(repoUrl)) {
      setName(inferName(url));
    }
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setError('');
    setDeploying(true);
    try {
      const payload: Parameters<typeof createApp>[0] = {
        name,
        start_command: startCmd,
      };
      if (mode === 'github' && repoUrl) {
        payload.repo_url = repoUrl;
        payload.branch = branch;
      }
      await createApp(payload);
      setName('');
      setStartCmd('');
      setRepoUrl('');
      setBranch('main');
      setShowForm(false);
      loadApps();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setDeploying(false);
    }
  }

  function handleLogout() {
    clearToken();
    navigate('/login');
  }

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
        <div className="page-header">
          <span className="page-title">Deployments</span>
          <button
            className={showForm ? 'btn btn-ghost btn-sm' : 'btn btn-primary btn-sm'}
            onClick={() => setShowForm(!showForm)}
          >
            {showForm ? 'Cancel' : 'New app'}
          </button>
        </div>

        {showForm && (
          <div className="deploy-form fade-in">
            <div className="deploy-tabs">
              <button
                className={`deploy-tab ${mode === 'github' ? 'active' : ''}`}
                onClick={() => setMode('github')}
                type="button"
              >
                Import Git Repository
              </button>
              <button
                className={`deploy-tab ${mode === 'manual' ? 'active' : ''}`}
                onClick={() => setMode('manual')}
                type="button"
              >
                Manual
              </button>
            </div>

            <form onSubmit={handleCreate}>
              {mode === 'github' && (
                <>
                  <div className="form-group">
                    <label>Repository URL</label>
                    <input
                      value={repoUrl}
                      onChange={e => handleRepoUrlChange(e.target.value)}
                      placeholder="https://github.com/you/your-repo"
                      autoFocus
                    />
                    <p className="form-hint">Private repos require a Git token in Settings</p>
                  </div>
                  <div className="flex gap-sm">
                    <div className="form-group" style={{ flex: 1 }}>
                      <label>App name</label>
                      <input
                        value={name}
                        onChange={e => setName(e.target.value)}
                        placeholder="my-app"
                      />
                    </div>
                    <div className="form-group" style={{ flex: 0, minWidth: 140 }}>
                      <label>Branch</label>
                      <input
                        value={branch}
                        onChange={e => setBranch(e.target.value)}
                        placeholder="main"
                      />
                    </div>
                  </div>
                </>
              )}

              {mode === 'manual' && (
                <div className="form-group">
                  <label>App name</label>
                  <input
                    value={name}
                    onChange={e => setName(e.target.value)}
                    placeholder="my-app"
                    autoFocus
                  />
                </div>
              )}

              <div className="form-group">
                <label>Start command</label>
                <input
                  value={startCmd}
                  onChange={e => setStartCmd(e.target.value)}
                  placeholder="npm start"
                />
              </div>

              {error && <p className="error-msg">{error}</p>}

              <button type="submit" className="btn btn-primary" disabled={deploying}>
                {deploying ? (
                  <>
                    <span className="spinner" />
                    {mode === 'github' ? 'Cloning...' : 'Creating...'}
                  </>
                ) : (
                  mode === 'github' ? 'Import & Deploy' : 'Create App'
                )}
              </button>
            </form>
          </div>
        )}

        {apps.length > 0 ? (
          <div className="app-list">
            {apps.map(app => (
              <Link to={`/apps/${app.id}`} key={app.id} className="app-row">
                <div className="app-info">
                  <span className="app-name">{app.name}</span>
                  <span className={`badge badge-${app.status}`}>{app.status}</span>
                  {app.repo_url && (
                    <span className="app-repo">{repoShortName(app.repo_url)}</span>
                  )}
                </div>
                <div className="app-meta">
                  <span>:{app.port}</span>
                </div>
              </Link>
            ))}
          </div>
        ) : !showForm && (
          <div className="empty-state">
            <p>No deployments yet</p>
            <button className="btn btn-primary btn-sm" onClick={() => setShowForm(true)}>
              Deploy your first app
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
