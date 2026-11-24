import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

const card = {
  player: {
    player_id: '11111111-1111-4111-8111-111111111111',
    display_name: 'Priya',
    age: 32,
    gender: 'woman',
    skill_level: 'intermediate',
    bio: 'Tuesday evening rallies.',
    play_summary: 'Tue, Thu evenings',
    photo_urls: [],
    distance_miles: 3
  }
};

test.beforeEach(async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 401, body: '' }));
});

test('landing page offers a passwordless sign-in', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: /find a real game/i })).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'E-mail' })).toBeVisible();
});

test('discover shows a card and creates a mutual match', async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/discovery?**', (route) =>
    route.fulfill({ json: { cards: [card], next_cursor: null } })
  );
  await page.route('**/api/v1/reactions', (route) =>
    route.fulfill({ json: { matched: true, match_id: '22222222-2222-4222-8222-222222222222' } })
  );
  await page.goto('/discover');
  await expect(page.getByRole('heading', { name: 'Priya, 32' })).toBeVisible();
  await page.getByRole('button', { name: 'Like' }).click();
  await expect(page.getByRole('heading', { name: /you both said yes/i })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Open match' })).toBeVisible();
});

test('offline state is announced', async ({ page }) => {
  await page.goto('/');
  await page.evaluate(() => window.dispatchEvent(new Event('offline')));
  await expect(page.getByRole('status').filter({ hasText: 'You are offline' })).toBeVisible();
});

test('landing page has no serious accessibility violations', async ({ page }) => {
  await page.goto('/');
  const results = await new AxeBuilder({ page }).analyze();
  const serious = results.violations.filter(
    (violation) => violation.impact === 'serious' || violation.impact === 'critical'
  );
  expect(serious).toEqual([]);
});
