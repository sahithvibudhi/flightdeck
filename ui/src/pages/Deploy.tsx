import { useState, useEffect, useRef } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  createApp, startApp, getAppLogs, getSystemInfo, uploadZip,
  errMsg, findInvalidEnv,
  type App, type EnvVar, type SystemInfo,
  replaceEnvs,
} from '../api';
import { FolderIcon, UploadIcon, GitHubIcon, EyeIcon, EyeOffIcon } from '../components/Icons';

type SourceType = 'path' | 'upload' | 'github';
type DeployPhase = 'form' | 'deploying' | 'running' | 'error';

export default function Deploy() {
  const navigate = useNavigate();
  const logRef = useRef<HTMLDivElement>(null);
  const [system, setSystem] = useState<SystemInfo | null>(null);

  const [phase, setPhase] = useState<DeployPhase>('form');

  const [source, setSource] = useState<SourceType>('github');
  const [repoUrl, setRepoUrl] = useState('');
  const [branch, setBranch] = useState('main');
  const [workDir, setWorkDir] = useState('');
  const [zipFile, setZipFile] = useState<File | null>(null);

  const [name, setName] = useState('');
  const [startCmd, setStartCmd] = useState('');
  const [buildCmd, setBuildCmd] = useState('');
  const [appPort, setAppPort] = useState('');
  const [healthPath, setHealthPath] = useState('');

  const [envs, setEnvs] = useState<EnvVar[]>([]);
  const [showEnvs, setShowEnvs] = useState(false);
  const [shownEnvValues, setShownEnvValues] = useState<Set<number>>(new Set());
  const [error, setError] = useState('');

  const [createdApp, setCreatedApp] = useState<App | null>(null);
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => {
    getSystemInfo().then(setSystem).catch(() => { /* transient */ });
  }, []);

  useEffect(() => {
    if (phase !== 'deploying' || !createdApp) return;
    const interval = setInterval(async () => {
      try {
        const res = await getAppLogs(createdApp.id);
        setLogs(res.lines);
      } catch { /* transient */ }
    }, 1000);
    return () => clearInterval(interval);
  }, [phase, createdApp]);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs]);

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

  const nameValid = /^[a-z0-9][a-z0-9-]{0,62}$/.test(name);

  function canDeploy(): boolean {
    if (!name || !nameValid || !startCmd) return false;
    if (source === 'github' && !repoUrl) return false;
    if (source === 'path' && !workDir) return false;
    if (source === 'upload' && !zipFile) return false;
    return true;
  }

  function addEnvRow() {
    setEnvs([...envs, { key: '', value: '' }]);
  }

  function updateEnv(index: number, field: 'key' | 'value', val: string) {
    const updated = [...envs];
    updated[index] = { ...updated[index], [field]: val };
    setEnvs(updated);
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

  async function handleDeploy() {
    setError('');
    setPhase('deploying');
    setLogs(['Deploying...']);

    try {
      const payload: Parameters<typeof createApp>[0] = {
        name,
        start_command: startCmd,
      };
      if (buildCmd) {
        payload.build_command = buildCmd;
      }
      if (appPort) {
        payload.port = parseInt(appPort, 10);
      }
      if (source === 'github' && repoUrl) {
        payload.repo_url = repoUrl;
        payload.branch = branch || 'main';
      }
      if (source === 'path' && workDir) {
        payload.work_dir = workDir;
      }
      if (healthPath) {
        payload.health_path = healthPath;
      }

      const app = await createApp(payload);
      setCreatedApp(app);
      setLogs(prev => [...prev, `App created on port ${app.port}`]);

      if (source === 'upload' && zipFile) {
        setLogs(prev => [...prev, 'Uploading zip file...']);
        await uploadZip(app.id, zipFile);
        setLogs(prev => [...prev, 'Zip extracted']);
      }

      const validEnvs = envs.filter(e => e.key.trim() !== '');
      const invalidEnv = findInvalidEnv(validEnvs);
      if (invalidEnv) {
        throw new Error(invalidEnv);
      }
      if (validEnvs.length > 0) {
        await replaceEnvs(app.id, validEnvs);
        setLogs(prev => [...prev, `Set ${validEnvs.length} environment variable(s)`]);
      }

      setLogs(prev => [...prev, 'Starting process...']);
      await startApp(app.id);
      setPhase('running');
    } catch (err) {
      setLogs(prev => [...prev, `Error: ${errMsg(err)}`]);
      setPhase('error');
    }
  }

  const installed = system?.runtimes.filter(r => r.installed) || [];

  if (phase !== 'form') {
    return (
      <div className="layout">
        <nav className="nav">
          <div className="nav-left">
            <Link to="/" className="nav-brand">flightdeck</Link>
          </div>
        </nav>
        <div className="deploy-state fade-in">
          <div className="deploy-state-name">{name}</div>

          <div className="deploy-state-log" ref={logRef}>
            {logs.join('\n')}
          </div>

          {phase === 'running' && (
            <>
              <div className="deploy-state-status">
                <span className="deploy-state-status-dot" style={{ background: 'var(--success)' }} />
                running
              </div>
              {createdApp && system?.server_ip && (
                <p className="deploy-state-url">
                  Your app is live at{' '}
                  <a href={`http://${system.server_ip}:${createdApp.url_port}`} target="_blank" rel="noreferrer">
                    http://{system.server_ip}:{createdApp.url_port}
                  </a>
                </p>
              )}
              <div className="deploy-state-actions">
                <Link to="/" className="btn btn-primary" style={{ textDecoration: 'none' }}>Go to dashboard</Link>
                {createdApp && (
                  <Link to={`/apps/${createdApp.id}`} className="btn btn-secondary" style={{ textDecoration: 'none' }}>Configure domain</Link>
                )}
              </div>
            </>
          )}

          {phase === 'error' && (
            <>
              <div className="deploy-state-status">
                <span className="deploy-state-status-dot" style={{ background: 'var(--error)' }} />
                <span style={{ color: 'var(--error)' }}>error</span>
              </div>
              <div className="deploy-state-actions">
                <button className="btn btn-primary" onClick={() => setPhase('form')}>Try again</button>
              </div>
            </>
          )}

          {phase === 'deploying' && (
            <div className="flex-center gap-sm">
              <span className="spinner" />
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="layout">
      <nav className="nav">
        <div className="nav-left">
          <Link to="/" className="nav-brand">flightdeck</Link>
        </div>
      </nav>
      <div className="deploy-wizard fade-in">
        <button className="deploy-back" onClick={() => navigate('/')}>
          ← Back
        </button>

        <div className="step-title">Deploy an app</div>
        <div className="step-subtitle">Choose a source, configure, and deploy</div>

        <div className="source-tabs">
          <button
            className={`source-tab ${source === 'path' ? 'source-tab-active' : ''}`}
            onClick={() => setSource('path')}
          >
            <FolderIcon /> Server Path
          </button>
          <button
            className={`source-tab ${source === 'upload' ? 'source-tab-active' : ''}`}
            onClick={() => setSource('upload')}
          >
            <UploadIcon /> Upload Zip
          </button>
          <button
            className={`source-tab ${source === 'github' ? 'source-tab-active' : ''}`}
            onClick={() => setSource('github')}
          >
            <GitHubIcon /> GitHub
          </button>
        </div>

        {source === 'path' && (
          <div className="form-group fade-in">
            <label htmlFor="deploy-workdir">Working directory</label>
            <input
              id="deploy-workdir"
              value={workDir}
              onChange={e => setWorkDir(e.target.value)}
              placeholder="/home/deploy/my-app"
              autoFocus
            />
            <p className="form-hint">Full path to a directory already on this server</p>
          </div>
        )}

        {source === 'upload' && (
          <div className="form-group fade-in">
            <label>Zip file</label>
            <div
              className={`upload-zone ${zipFile ? 'upload-zone-active' : ''}`}
              onClick={() => document.getElementById('zip-input')?.click()}
              onDragOver={e => e.preventDefault()}
              onDrop={e => {
                e.preventDefault();
                const f = e.dataTransfer.files[0];
                if (f && f.name.endsWith('.zip')) setZipFile(f);
              }}
            >
              {zipFile ? (
                <span>{zipFile.name} ({(zipFile.size / 1024 / 1024).toFixed(1)} MB)</span>
              ) : (
                <span>Drop a .zip file here or click to browse</span>
              )}
            </div>
            <input
              id="zip-input"
              type="file"
              accept=".zip"
              style={{ display: 'none' }}
              onChange={e => {
                const f = e.target.files?.[0];
                if (f) setZipFile(f);
              }}
            />
          </div>
        )}

        {source === 'github' && (
          <div className="fade-in">
            <div className="form-group">
              <label htmlFor="deploy-repo">Repository URL</label>
              <input
                id="deploy-repo"
                value={repoUrl}
                onChange={e => handleRepoUrlChange(e.target.value)}
                placeholder="https://github.com/user/repo"
                autoFocus
              />
              <p className="form-hint">Private repos require a <Link to="/settings" style={{ color: 'var(--text-secondary)' }}>token in Settings</Link></p>
            </div>
            <div className="form-group">
              <label htmlFor="deploy-branch">Branch</label>
              <input
                id="deploy-branch"
                value={branch}
                onChange={e => setBranch(e.target.value)}
                placeholder="main"
              />
            </div>
          </div>
        )}

        <hr style={{ border: 'none', borderTop: '1px solid var(--border)', margin: '20px 0' }} />

        <div className="form-group">
          <label htmlFor="deploy-name">App name</label>
          <input
            id="deploy-name"
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="my-app"
          />
          {name && !nameValid
            ? <p className="form-hint" style={{ color: 'var(--error)' }}>Lowercase letters, digits, and hyphens only</p>
            : <p className="form-hint">Lowercase letters, digits, and hyphens</p>}
        </div>

        <div className="form-group">
          <label htmlFor="deploy-start">Start command</label>
          <input
            id="deploy-start"
            value={startCmd}
            onChange={e => setStartCmd(e.target.value)}
            placeholder="node server.js"
          />
          {installed.length > 0 && (
            <p className="form-hint">Available: {installed.map(r => r.name.toLowerCase()).join(', ')}</p>
          )}
        </div>

        <div className="form-group">
          <label htmlFor="deploy-port">Port <span style={{ color: 'var(--text-tertiary)', fontWeight: 400 }}>(optional)</span></label>
          <input
            id="deploy-port"
            value={appPort}
            onChange={e => setAppPort(e.target.value.replace(/\D/g, ''))}
            placeholder="Auto-assigned if empty (e.g. 3000, 8080)"
          />
          <p className="form-hint">The port your app listens on. Leave empty to auto-assign.</p>
        </div>

        <div className="form-group">
          <label htmlFor="deploy-build">Build command <span style={{ color: 'var(--text-tertiary)', fontWeight: 400 }}>(optional)</span></label>
          <input
            id="deploy-build"
            value={buildCmd}
            onChange={e => setBuildCmd(e.target.value)}
            placeholder="npm install && npm run build"
          />
          <p className="form-hint">Runs before start command. Chain with &&</p>
        </div>

        <div className="form-group">
          <label htmlFor="deploy-health">Health check path <span style={{ color: 'var(--text-tertiary)', fontWeight: 400 }}>(optional)</span></label>
          <input
            id="deploy-health"
            value={healthPath}
            onChange={e => setHealthPath(e.target.value)}
            placeholder="/health"
          />
          <p className="form-hint">Enables zero-downtime deploys. Your app must listen on $PORT.</p>
        </div>

        <div style={{ marginTop: 24 }}>
          <button className="btn-text" onClick={() => setShowEnvs(!showEnvs)}>
            {showEnvs ? '− Hide' : '+ Show'} environment variables
          </button>

          {showEnvs && (
            <div className="fade-in" style={{ marginTop: 12 }}>
              {envs.map((env, i) => {

                const visible = shownEnvValues.has(i);
                return (
                <div key={i} className="env-row">
                  <input
                    placeholder="KEY"
                    value={env.key}
                    onChange={e => updateEnv(i, 'key', e.target.value)}
                    style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}
                  />
                  <input
                    placeholder="value"
                    value={env.value}
                    type={visible ? 'text' : 'password'}
                    onChange={e => updateEnv(i, 'value', e.target.value)}
                    style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}
                  />
                  <button
                    type = "button"
                    className = "btn btn-ghost btn-sm btn-icon"
                    onClick ={() => toggleEnvVisibility(i)}
                    title = {visible ? 'Hide value' : 'Show value'} >
                      {visible ? <EyeOffIcon/> : <EyeIcon/>}
                  </button>

                  <button type="button" className="btn btn-ghost btn-sm" onClick={() => removeEnvRow(i)} style={{ flexShrink: 0 }}>
                    ×
                  </button>
                </div>);
              })}
              <button type="button" className="btn-text" onClick={addEnvRow} style={{ marginTop: envs.length > 0 ? 8 : 0 }}>
                + Add variable
              </button>
            </div>
          )}
        </div>

        {error && <p className="error-msg">{error}</p>}

        <div className="deploy-actions">
          <button className="btn btn-primary" onClick={handleDeploy} disabled={!canDeploy()}>
            Deploy
          </button>
        </div>
      </div>
    </div>
  );
}
