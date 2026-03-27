import { request, type APIRequestContext, type APIResponse } from '@playwright/test';

const THROTTLE_MS = 700;
const MAX_RETRIES = 3;

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

  private async doWithRetry(fn: () => Promise<APIResponse>, retries = MAX_RETRIES): Promise<APIResponse> {
    for (let i = 0; i < retries; i++) {
      const res = await fn();
      if (res.status() === 429) {
        const wait = parseInt(res.headers()['retry-after'] || '2') * 1000 + 1000;
        console.warn(`Rate limited, waiting ${wait}ms...`);
        await new Promise(r => setTimeout(r, wait));
        continue;
      }
      return res;
    }
    throw new Error('Max retries exceeded due to rate limiting');
  }

  private async unwrap(res: APIResponse): Promise<any> {
    const body = await res.json();
    if (body.success === false) {
      throw new Error(`API error (${res.status()}): ${body.error?.message || JSON.stringify(body.error)}`);
    }
    return body.data ?? body;
  }
}

export async function createApiClient(baseURL: string, token: string = ''): Promise<ApiClient> {
  const ctx = await request.newContext();
  return new ApiClient(ctx, baseURL, token);
}
