import { useState, useEffect, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { listApps, createApp, clearToken, type App } from '../api';

export default function Apps() {
  const [apps, setApps] = useState<App[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState('');
  const [startCmd, setStartCmd] = useState('');
  const [error, setError] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    loadApps();
  }, []);

  async function loadApps() {
    try {
      setApps(await listApps());
    } catch {
      // handled by api.ts
    }
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await createApp(name, startCmd);
      setName('');
      setStartCmd('');
      setShowForm(false);
      loadApps();
    } catch (err: any) {
      setError(err.message);
    }
  }

  function handleLogout() {
    clearToken();
    navigate('/login');
  }

  return (
    <>
      <nav className="nav">
        <Link to="/" className="nav-brand">nestops</Link>
        <div className="nav-links">
          <Link to="/settings">Settings</Link>
          <a href="#" onClick={handleLogout}>Logout</a>
        </div>
      </nav>
      <div className="container">
        <div className="flex-between">
          <h1>Apps</h1>
          <button className="btn btn-primary" onClick={() => setShowForm(!showForm)}>
            {showForm ? 'Cancel' : 'Deploy new app'}
          </button>
        </div>

        {showForm && (
          <form className="card" onSubmit={handleCreate}>
            <div className="form-group">
              <label>App name</label>
              <input value={name} onChange={e => setName(e.target.value)} placeholder="my-app" />
            </div>
            <div className="form-group">
              <label>Start command</label>
              <input value={startCmd} onChange={e => setStartCmd(e.target.value)} placeholder="node server.js" />
            </div>
            {error && <p className="error-msg">{error}</p>}
            <button type="submit" className="btn btn-primary">Create</button>
          </form>
        )}

        {apps.map(app => (
          <Link to={`/apps/${app.id}`} key={app.id} className="app-row">
            <div className="app-info">
              <span className="app-name">{app.name}</span>
              <span className={`badge badge-${app.status}`}>{app.status}</span>
            </div>
            <span className="app-port">:{app.port}</span>
          </Link>
        ))}

        {apps.length === 0 && !showForm && (
          <p style={{ color: 'var(--text-muted)', textAlign: 'center', marginTop: '3rem' }}>
            No apps yet. Click "Deploy new app" to get started.
          </p>
        )}
      </div>
    </>
  );
}
