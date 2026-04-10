import { useState, useEffect, useRef } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  createApp, startApp, getAppLogs, getSystemInfo, uploadZip,
  type App, type EnvVar, type SystemInfo,
  replaceEnvs,
} from '../api';

type SourceType = 'path' | 'upload' | 'github';
type DeployPhase = 'form' | 'deploying' | 'running' | 'error';

function FolderIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
      <path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
    </svg>
  );
}

function UploadIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
      <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M17 8l-5-5-5 5M12 3v12" />
    </svg>
  );
}

function GitHubIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 2C6.477 2 2 6.477 2 12c0 4.42 2.865 8.166 6.839 9.489.5.092.682-.217.682-.482 0-.237-.009-.866-.013-1.7-2.782.604-3.369-1.34-3.369-1.34-.454-1.156-1.11-1.463-1.11-1.463-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.831.092-.646.35-1.086.636-1.337-2.22-.252-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0112 6.836c.85.004 1.705.114 2.504.336 1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C19.138 20.161 22 16.416 22 12c0-5.523-4.477-10-10-10z" />
    </svg>
  );
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

  const [envs, setEnvs] = useState<EnvVar[]>([]);
  const [showEnvs, setShowEnvs] = useState(false);
  const [shownEnvValues, setShownEnvValues] = useState<Set<number>>(new Set());
  const [error, setError] = useState('');

  const [createdApp, setCreatedApp] = useState<App | null>(null);
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => {
    getSystemInfo().then(setSystem).catch(() => {});
  }, []);

  useEffect(() => {
    if (phase !== 'deploying' || !createdApp) return;
    const interval = setInterval(async () => {
      try {
        const res = await getAppLogs(createdApp.id);
        setLogs(res.lines);
      } catch {}
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

  function canDeploy(): boolean {
    if (!name || !startCmd) return false;
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

  function toggleEnvVisibility(index: number){
    setShownEnvValues(prev =>{
      const next = new Set(prev);
      next.has(index) ? next.delete(index) : next.add(index);
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

      const app = await createApp(payload);
      setCreatedApp(app);
      setLogs(prev => [...prev, `App created on port ${app.port}`]);

      if (source === 'upload' && zipFile) {
        setLogs(prev => [...prev, 'Uploading zip file...']);
        await uploadZip(app.id, zipFile);
        setLogs(prev => [...prev, 'Zip extracted']);
      }

      const validEnvs = envs.filter(e => e.key.trim() !== '');
      if (validEnvs.length > 0) {
        await replaceEnvs(app.id, validEnvs);
        setLogs(prev => [...prev, `Set ${validEnvs.length} environment variable(s)`]);
      }

      setLogs(prev => [...prev, 'Starting process...']);
      await startApp(app.id);
      setPhase('running');
    } catch (err: any) {
      setLogs(prev => [...prev, `Error: ${err.message}`]);
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
                <span className="deploy-state-status-dot" style={{ background: '#fff' }} />
                running
              </div>
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
            <label>Working directory</label>
            <input
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
              <label>Repository URL</label>
              <input
                value={repoUrl}
                onChange={e => handleRepoUrlChange(e.target.value)}
                placeholder="https://github.com/user/repo"
                autoFocus
              />
              <p className="form-hint">Private repos require a <Link to="/settings" style={{ color: 'var(--text-secondary)' }}>token in Settings</Link></p>
            </div>
            <div className="form-group">
              <label>Branch</label>
              <input
                value={branch}
                onChange={e => setBranch(e.target.value)}
                placeholder="main"
              />
            </div>
          </div>
        )}

        <hr style={{ border: 'none', borderTop: '1px solid var(--border)', margin: '20px 0' }} />

        <div className="form-group">
          <label>App name</label>
          <input
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="my-app"
          />
          <p className="form-hint">Lowercase, no spaces</p>
        </div>

        <div className="form-group">
          <label>Start command</label>
          <input
            value={startCmd}
            onChange={e => setStartCmd(e.target.value)}
            placeholder="node server.js"
          />
          {installed.length > 0 && (
            <p className="form-hint">Available: {installed.map(r => r.name.toLowerCase()).join(', ')}</p>
          )}
        </div>

        <div className="form-group">
          <label>Port <span style={{ color: 'var(--text-tertiary)', fontWeight: 400 }}>(optional)</span></label>
          <input
            value={appPort}
            onChange={e => setAppPort(e.target.value.replace(/\D/g, ''))}
            placeholder="Auto-assigned if empty (e.g. 3000, 8080)"
          />
          <p className="form-hint">The port your app listens on. Leave empty to auto-assign.</p>
        </div>

        <div className="form-group">
          <label>Build command <span style={{ color: 'var(--text-tertiary)', fontWeight: 400 }}>(optional)</span></label>
          <input
            value={buildCmd}
            onChange={e => setBuildCmd(e.target.value)}
            placeholder="npm install && npm run build"
          />
          <p className="form-hint">Runs before start command. Chain with &&</p>
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
