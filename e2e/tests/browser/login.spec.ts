import { test, expect } from '@playwright/test';

const BASE = process.env.E2E_BASE_URL || 'https://halaos.com';

test.describe('Login Page', () => {
  test('login page loads with email and password inputs', async ({ page }) => {
    await page.goto(BASE + '/login');
    await page.waitForLoadState('load');

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
    await page.waitForLoadState('load');

    // Fill in credentials with a wrong password
    // Try data-testid first, fall back to NaiveUI form selectors (email input type is "email")
    const emailInput = page.locator('[data-testid="email-input"] input, .n-input input[type="email"]').first();
    await emailInput.fill('wrong@nonexistent.com');

    const passwordInput = page.locator('[data-testid="password-input"] input, .n-input input[type="password"]').first();
    await passwordInput.fill('WrongPassword123');

    // Click the login button
    const loginButton = page.locator('[data-testid="login-submit"], button:has-text("Login"), button:has-text("Sign in")').first();
    await loginButton.click();

    // Wait for error indicator: NMessage toast, NNotification, or inline validation
    const cssErrors = page.locator(
      '.n-message--error-type, .n-message--error, .n-message--warning-type, .n-message--warning, ' +
      '.n-notification, .n-form-item-feedback--error, .n-alert--error'
    );
    const textErrors = page.getByText(/invalid|incorrect|wrong|failed|error|too many/i);
    const errorMessage = cssErrors.or(textErrors).first();
    await expect(errorMessage).toBeVisible({ timeout: 15_000 });
  });
});
