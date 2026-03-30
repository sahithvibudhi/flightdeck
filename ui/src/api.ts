const BASE = '/api';

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

// Auth
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

// Apps
export interface App {
  id: string;
  name: string;
  port: number;
  start_command: string;
  status: string;
  domains: string[];
  created_at: string;
}

export const listApps = () => request<App[]>('/apps');
export const getApp = (id: string) => request<App>(`/apps/${id}`);
export const createApp = (name: string, start_command: string) =>
  request<App>('/apps', { method: 'POST', body: JSON.stringify({ name, start_command }) });
export const deleteApp = (id: string) => request(`/apps/${id}`, { method: 'DELETE' });
export const startApp = (id: string) => request(`/apps/${id}/start`, { method: 'POST' });
export const stopApp = (id: string) => request(`/apps/${id}/stop`, { method: 'POST' });
export const restartApp = (id: string) => request(`/apps/${id}/restart`, { method: 'POST' });
export const getAppLogs = (id: string, lines = 100) =>
  request<{ lines: string[] }>(`/apps/${id}/logs?lines=${lines}`);

// Envs
export interface EnvVar {
  key: string;
  value: string;
}

export const listEnvs = (appId: string) => request<EnvVar[]>(`/apps/${appId}/envs`);
export const replaceEnvs = (appId: string, envs: EnvVar[]) =>
  request(`/apps/${appId}/envs`, { method: 'PUT', body: JSON.stringify(envs) });

// Domains
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

// Settings
export interface Settings {
  panel_domain: string | null;
  admin_username: string;
}

export const getSettings = () => request<Settings>('/settings');
export const updatePanelDomain = (domain: string) =>
  request('/settings/domain', { method: 'PUT', body: JSON.stringify({ domain }) });
