import { createApiClient } from './api-client';

async function cleanup() {
  const baseURL = process.env.E2E_BASE_URL || 'https://halaos.com';
  console.log('E2E Cleanup');
  console.log('===========');
  console.log(`Target: ${baseURL}`);
  console.log('');
  console.log('Note: E2E test companies (E2E-Test-*) are isolated by company_id.');
  console.log('They do not affect production data and can be cleaned up via:');
  console.log('  - Direct database: DELETE FROM companies WHERE name LIKE \'E2E-Test-%\' AND created_at < NOW() - INTERVAL \'7 days\'');
  console.log('  - Admin API: When company deletion endpoint is available');
  console.log('');
  console.log('For now, old test companies are harmless — each run creates a new one.');
}

cleanup().catch(console.error);
