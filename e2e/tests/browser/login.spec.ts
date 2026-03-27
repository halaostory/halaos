import { test, expect } from '@playwright/test';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Login Page', () => {
  test('login page loads with email and password inputs', async ({ page }) => {
    await page.goto(BASE + '/login');
    await page.waitForLoadState('networkidle');

    // Should see the HalaOS branding
    await expect(page.locator('.logo-text')).toHaveText('HalaOS');

    // Email input should be visible
    const emailInput = page.locator('[data-testid="email-input"]');
    await expect(emailInput).toBeVisible();

    // Password input should be visible
    const passwordInput = page.locator('[data-testid="password-input"]');
    await expect(passwordInput).toBeVisible();

    // Login submit button should be visible
    const loginButton = page.locator('[data-testid="login-submit"]');
    await expect(loginButton).toBeVisible();
  });

  test('login with wrong password shows error message', async ({ page }) => {
    await page.goto(BASE + '/login');
    await page.waitForLoadState('networkidle');

    // Fill in credentials with a wrong password
    const emailInput = page.locator('[data-testid="email-input"] input');
    await emailInput.fill('wrong@nonexistent.com');

    const passwordInput = page.locator('[data-testid="password-input"] input');
    await passwordInput.fill('WrongPassword123');

    // Click the login button
    const loginButton = page.locator('[data-testid="login-submit"]');
    await loginButton.click();

    // Wait for error message to appear (NaiveUI message component renders in .n-message container)
    const errorMessage = page.locator('.n-message--error');
    await expect(errorMessage).toBeVisible({ timeout: 10_000 });
  });
});
