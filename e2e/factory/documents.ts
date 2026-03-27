import { ApiClient } from '../fixtures/api-client';

const CATEGORIES = [
  { name: 'Employment Contract', description: 'Employment agreements and contracts' },
  { name: 'Government IDs', description: 'SSS, PhilHealth, Pag-IBIG, TIN documents' },
  { name: 'Certifications', description: 'Professional certifications and training certificates' },
];

export async function seedDocuments(api: ApiClient): Promise<void> {
  let count = 0;
  for (const cat of CATEGORIES) {
    try {
      await api.post('/api/v1/201file/categories', cat);
      count++;
    } catch (err) {
      console.warn(`  Warning: Document category '${cat.name}' failed: ${err}`);
    }
  }
  console.log(`  Document categories: ${count}`);
}
