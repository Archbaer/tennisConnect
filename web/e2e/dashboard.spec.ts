import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

test.use({ colorScheme: 'light' });

const profile = {
  display_name: 'E2E Player',
  gender: 'man',
  skill_level: 'intermediate',
  ranking: null,
  bio: 'Test player.',
  gender_preference: 'all',
  min_age: 18,
  max_age: 120,
  skill_tolerance: 1,
  availability: [
    { weekday: 2, start: '18:00', end: '21:00' },
    { weekday: 4, start: '18:00', end: '20:00' }
  ],
  instagram_handle: null,
  age: 36
};

function daysAgo(days: number): string {
  return new Date(Date.now() - days * 86400000).toISOString();
}

const matches = [
  {
    match_id: '22222222-2222-4222-8222-222222222222',
    player_id: '11111111-1111-4111-8111-111111111111',
    display_name: 'Priya',
    matched_at: daysAgo(5),
    has_instagram: false,
    revealed: true,
    photo_urls: []
  },
  {
    match_id: '33333333-3333-4333-8333-333333333333',
    player_id: '44444444-4444-4444-8444-444444444444',
    display_name: 'Sam',
    matched_at: daysAgo(40),
    has_instagram: false,
    revealed: true,
    photo_urls: []
  },
  {
    match_id: '55555555-5555-4555-8555-555555555555',
    player_id: '66666666-6666-4666-8666-666666666666',
    display_name: 'Alex',
    matched_at: daysAgo(100),
    has_instagram: false,
    revealed: true,
    photo_urls: []
  }
];

test.beforeEach(async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/matches', (route) => route.fulfill({ json: { matches } }));
  await page.route('**/api/v1/profile', (route) => route.fulfill({ json: profile }));
});

test('dashboard renders KPIs, chart and recent matches', async ({ page }) => {
  await page.goto('/dashboard');

  await expect(page.getByRole('heading', { name: 'Overview', level: 1 })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Matches played' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Win rate' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Current streak' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Hours this month' })).toBeVisible();

  await expect(page.locator('svg.area-chart')).toBeVisible();
  await expect(page.locator('figcaption.chart-summary')).toContainText('Busiest month:');

  await expect(page.getByRole('table')).toBeVisible();
  await expect(page.getByRole('columnheader', { name: 'Opponent' })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'Priya', exact: true })).toBeVisible();
  await expect(page.getByText('Win', { exact: true }).or(page.getByText('Loss', { exact: true })).first()).toBeVisible();

  await expect(page.getByRole('heading', { name: 'Weekly play windows' })).toBeVisible();
  await expect(page.getByText('Tue evening')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Dashboard' }).first()).toBeVisible();
});

test('dashboard theme toggle persists and switches both themes', async ({ page }) => {
  await page.goto('/dashboard');
  await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'light');

  await page.getByRole('button', { name: 'Use dark theme' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'dark');
  expect(
    await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--paper').trim())
  ).toBe('#0b0f0d');
  expect(
    await page.locator('.site-header').evaluate((element) => getComputedStyle(element).backgroundColor)
  ).toContain('11, 15, 13');

  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'dark');

  await page.getByRole('button', { name: 'Use light theme' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-bs-theme', 'light');
  expect(
    await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--paper').trim())
  ).toBe('#f7f9f4');
});

test('dashboard has no Content-Security-Policy violations', async ({ page }) => {
  const violations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error' && /content security policy|refused to/i.test(message.text())) {
      violations.push(message.text());
    }
  });
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: 'Overview', level: 1 })).toBeVisible();
  expect(violations).toEqual([]);
});

test('dashboard has no serious accessibility violations', async ({ page }) => {
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: 'Overview', level: 1 })).toBeVisible();
  const results = await new AxeBuilder({ page }).analyze();
  const serious = results.violations.filter(
    (violation) => violation.impact === 'serious' || violation.impact === 'critical'
  );
  expect(serious).toEqual([]);
});

test('@mobile dashboard stacks without horizontal page overflow', async ({ page }) => {
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: 'Overview', level: 1 })).toBeVisible();
  await expect(page.locator('.dash-sidebar')).toBeHidden();
  await expect(page.getByRole('table')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)).toBe(false);
});
