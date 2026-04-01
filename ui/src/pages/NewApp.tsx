import { useState, useEffect, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { createApp, getSystemInfo, type SystemInfo } from '../api';

type DeployMode = 'github' | 'manual';

export default function NewApp() {
  const [system, setSystem] = useState<SystemInfo | null>(null);
  const [mode, setMode] = useState<DeployMode>('github');
  const [name, setName] = useState('');
  const [startCmd, setStartCmd] = useState('');
  const [repoUrl, setRepoUrl] = useState('');
  const [branch, setBranch] = useState('main');
  const [error, setError] = useState('');
  const [deploying, setDeploying] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    getSystemInfo().then(setSystem).catch(() => {});
  }, []);

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
      navigate('/');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setDeploying(false);
    }
  }

  const installed = system?.runtimes.filter(r => r.installed) || [];

  return (
    <div className="layout">
      <nav className="nav">
        <Link to="/" className="nav-brand">flightdeck</Link>
        <div className="nav-links">
          <Link to="/">Apps</Link>
          <Link to="/settings">Settings</Link>
        </div>
      </nav>
      <div className="container-narrow fade-in">
        <h1>New Deployment</h1>

        <div className="deploy-form">
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
              {installed.length > 0 && (
                <p className="form-hint">
                  Available: {installed.map(r => r.name.toLowerCase()).join(', ')}
                </p>
              )}
            </div>

            {error && <p className="error-msg">{error}</p>}

            <div className="flex gap-sm mt-sm">
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
              <Link to="/" className="btn" style={{ textDecoration: 'none' }}>Cancel</Link>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
