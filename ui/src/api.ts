const BASE = '/api';

export function errMsg(err: unknown): string {
  return err instanceof Error ? err.message : 'Something went wrong';
}

function getToken(): string | null {
  return localStorage.getItem('token');
}

export function setToken(token: string) {
  localStorage.setItem('token', token);
}

export function clearToken() {
  localStorage.removeItem('token');
}

export function isAuthenticated(): boolean {
  return !!getToken();
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  };

  const token = getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE}${path}`, { ...options, headers });

  if (res.status === 401) {
    clearToken();
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error || 'Request failed');
  }
  return data as T;
}

export const getSetupStatus = () =>
  request<{ needs_setup: boolean }>('/setup/status');

export const completeSetup = (username: string, password: string, domain?: string) =>
  request<{ token: string }>('/setup', {
    method: 'POST',
    body: JSON.stringify({ username, password, domain: domain || '' }),
  });

export const login = (username: string, password: string) =>
  request<{ token: string }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });

export const changePassword = (current: string, newPassword: string) =>
  request('/auth/password', {
    method: 'POST',
    body: JSON.stringify({ current, new: newPassword }),
  });

export interface App {
  id: string;
  name: string;
  port: number;
  // Port the app is actually serving on right now; after a zero-downtime
  // deploy this can be the standby port, so links should use it.
  url_port: number;
  start_command: string;
  build_command: string;
  work_dir: string;
  webhook_secret: string;
  health_path: string;
  status: string;
  repo_url: string | null;
  branch: string | null;
  domains: string[];
  cpu_percent: number;
  memory_mb: number;
  created_at: string;
}

export const listApps = () => request<App[]>('/apps');
export const getApp = (id: string) => request<App>(`/apps/${id}`);
export const deleteApp = (id: string) => request(`/apps/${id}`, { method: 'DELETE' });
export const startApp = (id: string) => request(`/apps/${id}/start`, { method: 'POST' });
export const stopApp = (id: string) => request(`/apps/${id}/stop`, { method: 'POST' });
export const restartApp = (id: string) => request(`/apps/${id}/restart`, { method: 'POST' });
export const pullApp = (id: string) => request<{ output: string }>(`/apps/${id}/pull`, { method: 'POST' });
export const getAppLogs = (id: string, lines = 100) =>
  request<{ lines: string[] }>(`/apps/${id}/logs?lines=${lines}`);

/*
Live log tail over Server-Sent Events. EventSource can't set headers,
so the JWT rides along as a query parameter.
*/
export function streamAppLogs(id: string): EventSource {
  const token = getToken() || '';
  return new EventSource(`${BASE}/apps/${id}/logs/stream?token=${encodeURIComponent(token)}`);
}

export interface Deployment {
  id: string;
  triggered_by: string;
  status: string;
  detail: string;
  commit_sha: string;
  commit_msg: string;
  started_at: string;
  finished_at: string | null;
}

export const listDeployments = (id: string) =>
  request<Deployment[]>(`/apps/${id}/deployments`);

export const deployApp = (id: string) =>
  request<{ deployment_id: string }>(`/apps/${id}/deploy`, { method: 'POST' });

export const rollbackDeployment = (appId: string, depId: string) =>
  request<{ deployment_id: string }>(`/apps/${appId}/deployments/${depId}/rollback`, { method: 'POST' });

export const createSampleApp = () => request<App>('/apps/sample', { method: 'POST' });

export const createApp = (data: {
  name: string;
  start_command: string;
  build_command?: string;
  port?: number;
  repo_url?: string;
  branch?: string;
  work_dir?: string;
  health_path?: string;
}) => request<App>('/apps', { method: 'POST', body: JSON.stringify(data) });

export const updateApp = (id: string, data: {
  name?: string;
  start_command?: string;
  build_command?: string;
  port?: number;
  repo_url?: string;
  branch?: string;
  work_dir?: string;
  health_path?: string;
}) => request<App>(`/apps/${id}`, { method: 'PUT', body: JSON.stringify(data) });

export async function uploadZip(appId: string, file: File): Promise<{ message: string }> {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${BASE}/apps/${appId}/upload`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: formData,
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Upload failed');
  return data;
}

export const installRuntime = (name: string) =>
  request<{ message: string; output: string }>('/system/install', {
    method: 'POST',
    body: JSON.stringify({ name }),
  });

export interface EnvVar {
  key: string;
  value: string;
}

/*
Mirrors the server-side rule: keys must be valid shell identifiers or
the .env file written at app start would be corrupt. Returns an error
message for the first invalid entry, or null.
*/
export function findInvalidEnv(envs: EnvVar[]): string | null {
  for (const e of envs) {
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(e.key)) {
      return `Invalid variable name "${e.key}": use letters, digits, and underscores, and don't start with a digit`;
    }
    if (/[\n\r]/.test(e.value)) {
      return `Value of "${e.key}" must not contain newlines`;
    }
  }
  return null;
}

export const listEnvs = (appId: string) => request<EnvVar[]>(`/apps/${appId}/envs`);
export const replaceEnvs = (appId: string, envs: EnvVar[]) =>
  request(`/apps/${appId}/envs`, { method: 'PUT', body: JSON.stringify(envs) });

export interface DomainEntry {
  id: string;
  domain: string;
}

export const listDomains = (appId: string) => request<DomainEntry[]>(`/apps/${appId}/domains`);
export const addDomain = (appId: string, domain: string) =>
  request<DomainEntry>(`/apps/${appId}/domains`, {
    method: 'POST',
    body: JSON.stringify({ domain }),
  });
export const removeDomain = (appId: string, domain: string) =>
  request(`/apps/${appId}/domains/${domain}`, { method: 'DELETE' });

export interface Settings {
  panel_domain: string | null;
  admin_username: string;
  has_git_token: boolean;
  notify_discord: string;
  notify_telegram_token: string;
  notify_telegram_chat: string;
  notify_webhook: string;
}

export const updateNotifications = (data: {
  discord: string;
  telegram_token: string;
  telegram_chat: string;
  webhook: string;
}) => request('/settings/notifications', { method: 'PUT', body: JSON.stringify(data) });

export const testNotifications = () =>
  request('/settings/notifications/test', { method: 'POST' });

export interface RuntimeInfo {
  name: string;
  version: string;
  installed: boolean;
}

export interface CaddyStatus {
  running: boolean;
  version?: string;
}

export interface SystemInfo {
  runtimes: RuntimeInfo[];
  caddy: CaddyStatus;
  os: string;
  arch: string;
  server_ip: string;
}

export const getSettings = () => request<Settings>('/settings');
export const updatePanelDomain = (domain: string) =>
  request('/settings/domain', { method: 'PUT', body: JSON.stringify({ domain }) });
export const updateGitToken = (token: string) =>
  request('/settings/git-token', { method: 'PUT', body: JSON.stringify({ token }) });
export const getSystemInfo = () => request<SystemInfo>('/system');

export interface ApiToken {
  id: string;
  name: string;
  scope: string;
  created_at: string;
  last_used: string | null;
}

export interface CreatedApiToken {
  id: string;
  name: string;
  scope: string;
  token: string;
}

export const listApiTokens = () => request<ApiToken[]>('/tokens');
export const createApiToken = (name: string, scope: string) =>
  request<CreatedApiToken>('/tokens', {
    method: 'POST',
    body: JSON.stringify({ name, scope }),
  });
export const deleteApiToken = (id: string) => request(`/tokens/${id}`, { method: 'DELETE' });

export interface ServerSnapshot {
  cpu_percent: number;
  memory_used_mb: number;
  memory_total_mb: number;
  disk_used_mb: number;
  disk_total_mb: number;
  timestamp: string;
}

export interface ServerMetricsHistory {
  snapshots: ServerSnapshot[];
}

export const getServerMetrics = () => request<ServerMetricsHistory>('/system/metrics');
