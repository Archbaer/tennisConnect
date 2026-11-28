import AxeBuilder from '@axe-core/playwright';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';

import { tokens } from './auth';

const signedInPages = [
  { path: '/discover', heading: 'Players near you' },
  { path: '/matches', heading: 'Matches' },
  { path: '/profile', heading: 'You' },
  { path: '/settings', heading: 'Settings' },
  { path: '/profile/edit', heading: 'Edit profile' },
  { path: '/profile/play-times', heading: 'Play times' },
  { path: '/profile/photos', heading: 'Photos' },
  { path: '/account/export', heading: 'Download your data' },
  { path: '/account/delete', heading: 'Delete account' }
] as const;

const signedOutPages = [
  { path: '/register', heading: 'Confirm your age' },
  { path: `/register?token=${'A'.repeat(43)}`, heading: 'Confirm your age' },
  { path: '/check-inbox', heading: 'Check your inbox' }
] as const;

async function useSession(context: BrowserContext) {
  await context.addCookies([
    {
      name: 'tennisconnect_session',
      value: tokens().player,
      url: 'http://127.0.0.1:4173',
      httpOnly: true,
      sameSite: 'Lax'
    }
  ]);
}

async function useTheme(page: Page, theme: 'light' | 'dark') {
  // initTheme() sets data-bs-theme in the layout's onMount, so its presence
  // proves hydration finished before axe runs.
  await expect(page.locator('html')).toHaveAttribute('data-bs-theme', /light|dark/);
  if ((await page.locator('html').getAttribute('data-bs-theme')) === theme) return;
  const name = theme === 'dark' ? 'Use dark theme' : 'Use light theme';
  await page.getByRole('button', { name }).click();
  await expect(page.locator('html')).toHaveAttribute('data-bs-theme', theme);
}

async function assertNoSeriousViolations(page: Page, label: string) {
  // Theme colours and page transitions animate; measure the settled frame.
  await page.evaluate(() =>
    Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => undefined)))
  );
  const results = await new AxeBuilder({ page }).analyze();
  const serious = results.violations.filter(
    (violation) => violation.impact === 'serious' || violation.impact === 'critical'
  );
  expect(
    serious,
    `${label}: ${JSON.stringify(
      serious.map((violation) => ({ id: violation.id, targets: violation.nodes.map((node) => node.target) }))
    )}`
  ).toEqual([]);
}

async function checkPageThemes(page: Page, path: string, heading: string | RegExp) {
  await page.goto(path);
  await expect(page.getByRole('heading', { level: 1, name: heading })).toBeVisible();

  await useTheme(page, 'light');
  await assertNoSeriousViolations(page, `${path} (light)`);

  await useTheme(page, 'dark');
  await assertNoSeriousViolations(page, `${path} (dark)`);

  await useTheme(page, 'light');
}

test.describe('signed-in axe coverage', () => {
  test.beforeEach(async ({ context }) => {
    await useSession(context);
  });

  for (const { path, heading } of signedInPages) {
    test(`${path} has no serious accessibility violations in light or dark`, async ({ page }) => {
      await checkPageThemes(page, path, heading);
    });
  }
});

test.describe('signed-out axe coverage', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/account/recovery', (route) =>
      route.fulfill({ json: { recover_until: '2026-12-24T12:00:00Z' } })
    );
  });

  for (const { path, heading } of signedOutPages) {
    test(`${path} has no serious accessibility violations in light or dark`, async ({ page }) => {
      await checkPageThemes(page, path, heading);
    });
  }

  test('/account/recovery has no serious accessibility violations in light or dark', async ({ page }) => {
    await checkPageThemes(page, '/account/recovery', /scheduled for deletion/i);
  });
});
