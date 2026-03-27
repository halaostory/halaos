import { request, type APIRequestContext, type APIResponse } from '@playwright/test';

const THROTTLE_MS = 1200; // 1.2s between requests to stay under 100 req/min rate limit
const MAX_RETRIES = 2;
const MAX_WAIT_MS = 3_000; // Default max wait per retry (3s) — keeps tests fast

let lastRequestTime = 0;

async function throttle(): Promise<void> {
  const now = Date.now();
  const elapsed = now - lastRequestTime;
  if (elapsed < THROTTLE_MS) {
    await new Promise(r => setTimeout(r, THROTTLE_MS - elapsed));
  }
  lastRequestTime = Date.now();
}

export class ApiClient {
  private ctx: APIRequestContext;
  private token: string;
  private baseURL: string;

  constructor(ctx: APIRequestContext, baseURL: string, token: string = '') {
    this.ctx = ctx;
    this.baseURL = baseURL;
    this.token = token;
  }

  setToken(token: string) {
    this.token = token;
  }

  private headers(): Record<string, string> {
    const h: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.token) h['Authorization'] = `Bearer ${this.token}`;
    return h;
  }

  async get(path: string, params?: Record<string, string>): Promise<any> {
    await throttle();
    const url = new URL(this.baseURL + path);
    if (params) Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v));
    const res = await this.doWithRetry(() =>
      this.ctx.get(url.toString(), { headers: this.headers() })
    );
    return this.unwrap(res);
  }

  async post(path: string, data?: any): Promise<any> {
    await throttle();
    const res = await this.doWithRetry(() =>
      this.ctx.post(this.baseURL + path, { headers: this.headers(), data })
    );
    return this.unwrap(res);
  }

  async put(path: string, data?: any): Promise<any> {
    await throttle();
    const res = await this.doWithRetry(() =>
      this.ctx.put(this.baseURL + path, { headers: this.headers(), data })
    );
    return this.unwrap(res);
  }

  async patch(path: string, data?: any): Promise<any> {
    await throttle();
    const res = await this.doWithRetry(() =>
      this.ctx.patch(this.baseURL + path, { headers: this.headers(), data })
    );
    return this.unwrap(res);
  }

  async delete(path: string): Promise<any> {
    await throttle();
    const res = await this.doWithRetry(() =>
      this.ctx.delete(this.baseURL + path, { headers: this.headers() })
    );
    return this.unwrap(res);
  }

  async postForm(path: string, formData: any): Promise<any> {
    await throttle();
    const res = await this.doWithRetry(() =>
      this.ctx.post(this.baseURL + path, {
        headers: { Authorization: `Bearer ${this.token}` },
        multipart: formData,
      })
    );
    return this.unwrap(res);
  }

  async getRaw(path: string, params?: Record<string, string>): Promise<APIResponse> {
    await throttle();
    const url = new URL(this.baseURL + path);
    if (params) Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v));
    return this.doWithRetry(() =>
      this.ctx.get(url.toString(), { headers: this.headers() })
    );
  }

  async dispose(): Promise<void> {
    await this.ctx.dispose();
  }

  /** Allow callers to set higher retry budget (e.g. globalSetup can wait minutes) */
  setRetryConfig(retries: number, maxWaitMs: number): void {
    this._retries = retries;
    this._maxWaitMs = maxWaitMs;
  }
  private _retries = MAX_RETRIES;
  private _maxWaitMs = MAX_WAIT_MS;

  private async doWithRetry(fn: () => Promise<APIResponse>): Promise<APIResponse> {
    for (let i = 0; i < this._retries; i++) {
      const res = await fn();
      if (res.status() === 429) {
        const retryAfter = parseInt(res.headers()['retry-after'] || '2');
        const wait = Math.min(retryAfter * 1000, this._maxWaitMs) + 1000;
        console.warn(`Rate limited (retry-after: ${retryAfter}s), waiting ${Math.round(wait / 1000)}s [attempt ${i + 1}/${this._retries}]...`);
        await new Promise(r => setTimeout(r, wait));
        continue;
      }
      return res;
    }
    throw new Error('Max retries exceeded due to rate limiting');
  }

  private async unwrap(res: APIResponse): Promise<any> {
    const body = await res.json();
    if (!res.ok() || body.success === false) {
      throw new Error(`API error (${res.status()}): ${body.error?.message || JSON.stringify(body.error || body)}`);
    }
    return body.data ?? body;
  }
}

export async function createApiClient(baseURL: string, token: string = ''): Promise<ApiClient> {
  const ctx = await request.newContext();
  return new ApiClient(ctx, baseURL, token);
}
