import { useState, useEffect, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { getSettings, updatePanelDomain, changePassword } from '../api';

export default function Settings() {
  const [domain, setDomain] = useState('');
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [msg, setMsg] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    loadSettings();
  }, []);

  async function loadSettings() {
    try {
      const s = await getSettings();
      setDomain(s.panel_domain || '');
    } catch {}
  }

  async function handleDomain(e: FormEvent) {
    e.preventDefault();
    setError('');
    setMsg('');
    try {
      await updatePanelDomain(domain);
      setMsg('Domain updated');
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function handlePassword(e: FormEvent) {
    e.preventDefault();
    setError('');
    setMsg('');
    try {
      await changePassword(currentPw, newPw);
      setCurrentPw('');
      setNewPw('');
      setMsg('Password updated');
    } catch (err: any) {
      setError(err.message);
    }
  }

  return (
    <>
      <nav className="nav">
        <Link to="/" className="nav-brand">nestops</Link>
        <div className="nav-links">
          <Link to="/">Apps</Link>
        </div>
      </nav>
      <div className="container">
        <h1>Settings</h1>

        {msg && <p style={{ color: 'var(--success)', marginBottom: '1rem' }}>{msg}</p>}
        {error && <p className="error-msg" style={{ marginBottom: '1rem' }}>{error}</p>}

        <div className="card section">
          <h2>Control Panel Domain</h2>
          <form onSubmit={handleDomain}>
            <div className="form-group">
              <label>Domain (leave blank for IP-only mode)</label>
              <input value={domain} onChange={e => setDomain(e.target.value)} placeholder="admin.example.com" />
            </div>
            <button type="submit" className="btn btn-primary">Update domain</button>
          </form>
        </div>

        <div className="card section">
          <h2>Change Password</h2>
          <form onSubmit={handlePassword}>
            <div className="form-group">
              <label>Current password</label>
              <input type="password" value={currentPw} onChange={e => setCurrentPw(e.target.value)} />
            </div>
            <div className="form-group">
              <label>New password</label>
              <input type="password" value={newPw} onChange={e => setNewPw(e.target.value)} />
            </div>
            <button type="submit" className="btn btn-primary">Change password</button>
          </form>
        </div>
      </div>
    </>
  );
}
