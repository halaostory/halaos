import { ApiClient } from '../fixtures/api-client';

const ANNOUNCEMENTS = [
  { title: 'Welcome E2E Test Company', content: 'This is a test announcement for E2E testing.', priority: 'normal' },
  { title: 'Important Notice', content: 'Please review the updated policies.', priority: 'high' },
];

export async function seedAnnouncements(api: ApiClient): Promise<void> {
  let count = 0;
  for (const ann of ANNOUNCEMENTS) {
    try {
      await api.post('/api/v1/announcements', ann);
      count++;
    } catch (err) {
      console.warn(`  Warning: Announcement '${ann.title}' failed: ${err}`);
    }
  }
  console.log(`  Announcements: ${count}`);
}
