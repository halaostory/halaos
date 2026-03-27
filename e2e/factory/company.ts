import { ApiClient } from '../fixtures/api-client';
import { updateState } from '../fixtures/state';

export async function seedCompany(api: ApiClient): Promise<void> {
  const ts = Date.now();
  const email = `admin-e2e-${ts}@test.halaos.com`;
  const password = `E2eTest${ts}!`;
  const companyName = `E2E-Test-${new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)}`;

  // Register (cli-register bypasses email verification)
  await api.post('/api/v1/auth/cli-register', {
    email,
    password,
    first_name: 'E2E',
    last_name: 'Admin',
    company_name: companyName,
  });

  // Login
  const loginRes = await api.post('/api/v1/auth/cli-login', { email, password });
  const token = loginRes.token || loginRes.access_token;
  const refreshToken = loginRes.refresh_token || '';
  if (!token) throw new Error('cli-login did not return token');

  api.setToken(token);
  updateState({ companyName, adminToken: token, adminRefreshToken: refreshToken, adminEmail: email, adminPassword: password });
  console.log(`  Company created: ${companyName}`);
}
