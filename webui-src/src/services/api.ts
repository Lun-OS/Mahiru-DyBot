import { Account, AccountInfo, Settings, Adapter } from '@/types';

const API_BASE = '/api/webui';

class ApiClient {
  private token: string | null = null;

  constructor() {
    this.token = localStorage.getItem('mahiru_dybot_token');
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...options.headers as Record<string, string>,
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    });

    if (response.status === 401) {
      this.clearToken();
      window.dispatchEvent(new Event('auth-expired'));
      throw new Error('Unauthorized');
    }

    return response.json();
  }

  async get<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET' });
  }

  async post<T>(endpoint: string, body?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' });
  }

  // Auth methods
  async checkInit(): Promise<{ initialized: boolean; authenticated: boolean }> {
    return this.get<{ initialized: boolean; authenticated: boolean }>('/me');
  }

  async setup(data: { password: string }): Promise<{ ok: boolean; token: string }> {
    const res = await this.post<{ ok: boolean; token?: string; error?: string }>(
      '/auth/setup',
      data
    );
    if (res.ok && res.token) {
      this.setToken(res.token);
      return { ok: true, token: res.token };
    }
    throw new Error(res.error || 'Setup failed');
  }

  async login(data: { password: string }): Promise<{ ok: boolean; token: string }> {
    const res = await this.post<{ ok: boolean; token?: string; error?: string }>(
      '/auth/login',
      data
    );
    if (res.ok && res.token) {
      this.setToken(res.token);
      return { ok: true, token: res.token };
    }
    throw new Error(res.error || 'Login failed');
  }

  async logout(): Promise<void> {
    try {
      await this.post('/auth/logout');
    } catch {
      // ignore
    }
    this.clearToken();
  }

  async verifyToken(): Promise<boolean> {
    try {
      const res = await this.post<{ ok: boolean }>('/auth/verify');
      return res.ok;
    } catch {
      return false;
    }
  }

  // Account methods
  async getAccounts(): Promise<Account[]> {
    const res = await this.get<{ ok: boolean; accounts: Account[] }>('/accounts');
    return res.accounts || [];
  }

  async getAccountInfo(id: string): Promise<AccountInfo> {
    const res = await this.get<{ ok: boolean; account: AccountInfo; error?: string }>(
      `/accounts/${id}/info`
    );
    if (res.ok) return res.account;
    throw new Error(res.error || 'Failed to get account info');
  }

  async createAccount(name: string): Promise<Account> {
    const res = await this.post<{ ok: boolean; account: Account; error?: string }>(
      '/accounts',
      { name }
    );
    if (res.ok) return res.account;
    throw new Error(res.error || 'Failed to create account');
  }

  async deleteAccount(id: string): Promise<void> {
    await this.delete(`/accounts/${id}`);
  }

  async renameAccount(id: string, name: string): Promise<void> {
    await this.post(`/accounts/${id}/rename`, { name });
  }

  async startAccount(id: string): Promise<void> {
    const res = await this.post<{ ok: boolean; error?: string }>(`/accounts/${id}/start`);
    if (!res.ok) throw new Error(res.error || 'Failed to start');
  }

  async stopAccount(id: string): Promise<void> {
    const res = await this.post<{ ok: boolean; error?: string }>(`/accounts/${id}/stop`);
    if (!res.ok) throw new Error(res.error || 'Failed to stop');
  }

  async getQRCode(id: string): Promise<{ ok: boolean; image_base64: string; token: string }> {
    const res = await this.get<{ ok: boolean; image_base64?: string; token?: string; error?: string }>(
      `/accounts/${id}/qrcode`
    );
    if (res.ok) return res as { ok: boolean; image_base64: string; token: string };
    throw new Error(res.error || 'Failed to get QR code');
  }

  async waitLogin(id: string, timeout = 300): Promise<{ ok: boolean; logged_in: boolean }> {
    const res = await this.get<{ ok: boolean; logged_in?: boolean; error?: string }>(
      `/accounts/${id}/wait-login?timeout=${timeout}`
    );
    if (res.ok) return res as { ok: boolean; logged_in: boolean };
    throw new Error(res.error || 'Login wait failed');
  }

  async updateAccountToken(id: string, token: string): Promise<void> {
    await this.post(`/accounts/${id}/token`, { token });
  }

  async updateAccountSettings(
    id: string,
    settings: { name?: string; viewport_width?: number; viewport_height?: number; custom_ua?: string }
  ): Promise<{ ok: boolean; error?: string }> {
    return this.post<{ ok: boolean; error?: string }>(`/accounts/${id}/settings`, settings);
  }

  // Debug methods
  async getScreenshot(id: string): Promise<Blob> {
    const response = await fetch(`${API_BASE}/accounts/${id}/screenshot`, {
      headers: this.token ? { Authorization: `Bearer ${this.token}` } : {},
    });
    if (!response.ok) {
      throw new Error('Failed to get screenshot');
    }
    return response.blob();
  }

  async evalJS(id: string, code: string): Promise<unknown> {
    const res = await this.post<{ ok: boolean; data?: unknown; error?: string }>(
      `/accounts/${id}/eval`,
      { js: code }
    );
    return res.data;
  }

  async getPageState(id: string): Promise<unknown> {
    const res = await this.get<{ ok: boolean; data?: unknown }>(`/accounts/${id}/console`);
    return res.data;
  }

  async getViewport(id: string): Promise<{ width: number; height: number }> {
    const res = await this.get<{ ok: boolean; width: number; height: number }>(
      `/accounts/${id}/viewport`
    );
    return { width: res.width, height: res.height };
  }

  async click(id: string, x: number, y: number): Promise<void> {
    await this.post(`/accounts/${id}/click`, { x, y });
  }

  async typeText(id: string, x: number, y: number, text: string): Promise<void> {
    await this.post(`/accounts/${id}/type`, { x, y, text });
  }

  async pressKey(id: string, key: string): Promise<void> {
    await this.post(`/accounts/${id}/key`, { key });
  }

  async scroll(id: string, x: number, y: number, deltaX: number, deltaY: number): Promise<void> {
    await this.post(`/accounts/${id}/scroll`, { x, y, delta_x: deltaX, delta_y: deltaY });
  }

  async rightClick(id: string, x: number, y: number): Promise<void> {
    await this.post(`/accounts/${id}/rightclick`, { x, y });
  }

  async getSystemInfo(): Promise<any> {
    return this.get<any>('/system/info');
  }

  // Settings methods
  async getSettings(): Promise<Settings> {
    const res = await this.get<{
      ok: boolean;
      onebot_access_token: string;
      screenshot_max_fps: number;
      jpeg_quality: number;
      reverse_ws: unknown[];
    }>('/settings');
    return {
      listen_addr: '',
      onebot_access_token: res.onebot_access_token || '',
      screenshot_max_fps: res.screenshot_max_fps || 10,
      jpeg_quality: res.jpeg_quality || 60,
      reverse_ws: (res.reverse_ws || []) as Settings['reverse_ws'],
    };
  }

  async updateSettings(settings: Record<string, unknown>): Promise<void> {
    await this.post('/settings', settings);
  }

  async getGlobalToken(): Promise<{ token: string }> {
    const res = await this.get<{ ok: boolean; token: string }>('/settings');
    return { token: res.token || '' };
  }

  async setGlobalToken(token: string): Promise<void> {
    await this.post('/settings', { token });
  }

  async resetPassword(
    currentPassword: string,
    newPassword: string
  ): Promise<{ ok: boolean; error?: string }> {
    return this.post<{ ok: boolean; error?: string }>('/auth/reset', {
      current_password: currentPassword,
      new_password: newPassword,
    });
  }

  async getVersion(): Promise<string> {
    return '1.0.0';
  }

  // Adapter methods (per-account)
  async getAccountAdapters(accountId: string): Promise<Adapter[]> {
    const res = await this.get<{ ok: boolean; adapters: Adapter[] }>(
      `/accounts/${accountId}/adapters`
    );
    return res.adapters || [];
  }

  async createAccountAdapter(accountId: string, adapter: Partial<Adapter>): Promise<Adapter> {
    const res = await this.post<{ ok: boolean; adapter: Adapter; error?: string }>(
      `/accounts/${accountId}/adapters`,
      adapter
    );
    if (res.ok) return res.adapter;
    throw new Error(res.error || 'Failed to create adapter');
  }

  async updateAccountAdapter(
    accountId: string,
    adapterId: string,
    update: Partial<Adapter>
  ): Promise<Adapter> {
    const res = await fetch(`${API_BASE}/accounts/${accountId}/adapters/${adapterId}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
      },
      body: JSON.stringify(update),
    }).then((r) => r.json());
    if (res.ok) return res.adapter;
    throw new Error(res.error || 'Failed to update adapter');
  }

  async deleteAccountAdapter(accountId: string, adapterId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/accounts/${accountId}/adapters/${adapterId}`, {
      method: 'DELETE',
      headers: this.token ? { Authorization: `Bearer ${this.token}` } : {},
    }).then((r) => r.json());
    if (!res.ok) throw new Error(res.error || 'Failed to delete adapter');
  }

  // Log methods
  async getAccountLogs(accountId: string, limit = 100): Promise<string[]> {
    const res = await this.get<{ ok: boolean; logs: string[] }>(
      `/accounts/${accountId}/logs?limit=${limit}`
    );
    return res.logs || [];
  }

  // Token management
  setToken(token: string): void {
    this.token = token;
    localStorage.setItem('mahiru_dybot_token', token);
  }

  clearToken(): void {
    this.token = null;
    localStorage.removeItem('mahiru_dybot_token');
  }

  getToken(): string | null {
    return this.token;
  }
}

export const api = new ApiClient();
export default api;
