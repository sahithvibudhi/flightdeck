import { useState, useEffect, useRef } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  createApp, startApp, getAppLogs, getSystemInfo,
  type App, type EnvVar, type SystemInfo,
  replaceEnvs,
} from '../api';

type SourceType = 'local' | 'github';
type WizardStep = 1 | 2 | 3;
type DeployPhase = 'wizard' | 'deploying' | 'running' | 'error';

const STEPS = [
  { num: 1, label: 'Source' },
  { num: 2, label: 'Config' },
  { num: 3, label: 'Environment' },
];

function FolderIcon() {
  return (
    <svg className="source-card-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
      <path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
    </svg>
  );
}

function GitHubIcon() {
  return (
    <svg className="source-card-icon" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 2C6.477 2 2 6.477 2 12c0 4.42 2.865 8.166 6.839 9.489.5.092.682-.217.682-.482 0-.237-.009-.866-.013-1.7-2.782.604-3.369-1.34-3.369-1.34-.454-1.156-1.11-1.463-1.11-1.463-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.831.092-.646.35-1.086.636-1.337-2.22-.252-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0112 6.836c.85.004 1.705.114 2.504.336 1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C19.138 20.161 22 16.416 22 12c0-5.523-4.477-10-10-10z" />
    </svg>
  );
}

export default function Deploy() {
  const navigate = useNavigate();
  const logRef = useRef<HTMLDivElement>(null);
  const [system, setSystem] = useState<SystemInfo | null>(null);

  const [step, setStep] = useState<WizardStep>(1);
  const [phase, setPhase] = useState<DeployPhase>('wizard');

  const [source, setSource] = useState<SourceType>('github');
  const [repoUrl, setRepoUrl] = useState('');
  const [workDir, setWorkDir] = useState('');

  const [name, setName] = useState('');
  const [startCmd, setStartCmd] = useState('');

  const [envs, setEnvs] = useState<EnvVar[]>([]);
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

  function canAdvance(): boolean {
    if (step === 1) {
      return source === 'github' ? repoUrl.length > 0 : workDir.length > 0;
    }
    if (step === 2) {
      return name.length > 0 && startCmd.length > 0;
    }
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

  async function handleDeploy() {
    setError('');
    setPhase('deploying');
    setLogs(['Deploying...']);

    try {
      const payload: Parameters<typeof createApp>[0] = {
        name,
        start_command: startCmd,
      };
      if (source === 'github' && repoUrl) {
        payload.repo_url = repoUrl;
        payload.branch = 'main';
      }

      const app = await createApp(payload);
      setCreatedApp(app);
      setLogs(prev => [...prev, `App created on port ${app.port}`]);

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

  if (phase !== 'wizard') {
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
                <button className="btn btn-primary" onClick={() => { setPhase('wizard'); setStep(2); }}>Try again</button>
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

        <div className="step-indicator">
          {STEPS.map((s, i) => (
            <div key={s.num} style={{ display: 'flex', alignItems: 'center' }}>
              {i > 0 && <div className="step-line" />}
              <div className={`step-item ${step === s.num ? 'step-item-active' : step > s.num ? 'step-item-done' : ''}`}>
                <span className="step-num">{s.num}</span>
                <span>{s.label}</span>
              </div>
            </div>
          ))}
        </div>

        {step === 1 && (
          <div className="fade-in">
            <div className="step-title">Where is your app?</div>
            <div className="step-subtitle">Choose how to import your project</div>

            <div className="source-cards">
              <div
                className={`source-card ${source === 'local' ? 'source-card-selected' : ''}`}
                onClick={() => setSource('local')}
              >
                <FolderIcon />
                <div className="source-card-label">Local directory</div>
                <div className="source-card-desc">App is already on this server</div>
              </div>
              <div
                className={`source-card ${source === 'github' ? 'source-card-selected' : ''}`}
                onClick={() => setSource('github')}
              >
                <GitHubIcon />
                <div className="source-card-label">GitHub</div>
                <div className="source-card-desc">Pull from a repository</div>
              </div>
            </div>

            {source === 'local' && (
              <div className="form-group fade-in">
                <label>Working directory</label>
                <input
                  value={workDir}
                  onChange={e => setWorkDir(e.target.value)}
                  placeholder="/home/deploy/my-app"
                  autoFocus
                />
              </div>
            )}

            {source === 'github' && (
              <div className="form-group fade-in">
                <label>Repository URL</label>
                <input
                  value={repoUrl}
                  onChange={e => handleRepoUrlChange(e.target.value)}
                  placeholder="https://github.com/user/repo"
                  autoFocus
                />
                <p className="form-hint">Private repos require a token in Settings</p>
              </div>
            )}

            <div className="deploy-actions">
              <button className="btn btn-primary" onClick={() => setStep(2)} disabled={!canAdvance()}>
                Next
              </button>
            </div>
          </div>
        )}

        {step === 2 && (
          <div className="fade-in">
            <div className="step-title">Configure your app</div>
            <div className="step-subtitle">Set the basics for your deployment</div>

            <div className="form-group">
              <label>App name</label>
              <input
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="my-app"
                autoFocus
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

            <div className="deploy-actions">
              <button className="btn btn-secondary" onClick={() => setStep(1)}>Back</button>
              <button className="btn btn-primary" onClick={() => setStep(3)} disabled={!canAdvance()}>
                Next
              </button>
            </div>
          </div>
        )}

        {step === 3 && (
          <div className="fade-in">
            <div className="step-title">Environment variables</div>
            <div className="step-subtitle">Optional — you can add these later.</div>

            {envs.map((env, i) => (
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
                  onChange={e => updateEnv(i, 'value', e.target.value)}
                  style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}
                />
                <button type="button" className="btn btn-ghost btn-sm" onClick={() => removeEnvRow(i)} style={{ flexShrink: 0 }}>
                  ×
                </button>
              </div>
            ))}

            <button type="button" className="btn-text" onClick={addEnvRow} style={{ marginTop: envs.length > 0 ? 8 : 0 }}>
              + Add variable
            </button>

            {error && <p className="error-msg">{error}</p>}

            <div className="deploy-actions">
              <button className="btn btn-secondary" onClick={() => setStep(2)}>Back</button>
              <button className="btn-text" onClick={handleDeploy}>Skip</button>
              <button className="btn btn-primary" onClick={handleDeploy}>Deploy</button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
