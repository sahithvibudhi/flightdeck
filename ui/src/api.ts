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
  start_command: string;
  build_command: string;
  work_dir: string;
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

export const createApp = (data: {
  name: string;
  start_command: string;
  build_command?: string;
  port?: number;
  repo_url?: string;
  branch?: string;
  work_dir?: string;
}) => request<App>('/apps', { method: 'POST', body: JSON.stringify(data) });

export const updateApp = (id: string, data: {
  name?: string;
  start_command?: string;
  build_command?: string;
  port?: number;
  repo_url?: string;
  branch?: string;
  work_dir?: string;
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
}

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
}

export const getSettings = () => request<Settings>('/settings');
export const updatePanelDomain = (domain: string) =>
  request('/settings/domain', { method: 'PUT', body: JSON.stringify({ domain }) });
export const updateGitToken = (token: string) =>
  request('/settings/git-token', { method: 'PUT', body: JSON.stringify({ token }) });
export const getSystemInfo = () => request<SystemInfo>('/system');

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
