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

const match = {
  match_id: '22222222-2222-4222-8222-222222222222',
  player_id: card.player.player_id,
  display_name: card.player.display_name,
  matched_at: new Date().toISOString(),
  has_instagram: false,
  revealed: false,
  photo_urls: []
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
  await page.route('**/api/v1/matches/22222222-2222-4222-8222-222222222222/reveal', (route) =>
    route.fulfill({ status: 204, body: '' })
  );
  await page.route('**/api/v1/matches/22222222-2222-4222-8222-222222222222', (route) =>
    route.fulfill({ json: match })
  );
  await page.goto('/discover');
  await expect(page.getByRole('heading', { name: 'Priya, 32' })).toBeVisible();
  await page.getByRole('button', { name: "I'd play" }).click();
  await expect(page).toHaveURL(/\/matches\/22222222-2222-4222-8222-222222222222\/new$/);
  await expect(page.getByRole('heading', { name: /you're both in/i })).toBeFocused();
  await expect(page.getByRole('status').filter({ hasText: "It's a match. You and Priya both said yes." })).toBeVisible();
});

test('new matches are marked in the list and tab', async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/matches', (route) =>
    route.fulfill({ json: { matches: [{ ...match, matched_at: new Date().toISOString() }] } })
  );
  await page.goto('/matches');
  await expect(page.getByText('New match')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Matches, new match' }).first()).toBeVisible();
});

test('discover opens the full player detail', async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/discovery?**', (route) =>
    route.fulfill({ json: { cards: [card], next_cursor: null } })
  );
  await page.route('**/api/v1/discovery/11111111-1111-4111-8111-111111111111?**', (route) =>
    route.fulfill({
      json: {
        player: {
          ...card.player,
          availability: [
            { weekday: 2, start: '18:00', end: '21:00' },
            { weekday: 4, start: '18:00', end: '21:00' }
          ]
        }
      }
    })
  );
  await page.goto('/discover');
  await page.getByRole('link', { name: 'View full profile' }).click();
  await expect(page).toHaveURL(/\/discover\/11111111-1111-4111-8111-111111111111\?radius=10$/);
  await expect(page.getByRole('heading', { name: 'Priya, 32' })).toBeVisible();
  await expect(page.getByText('Tue 18:00–21:00')).toBeVisible();
});

test('discover safety dialog names the player and offers block', async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/discovery?**', (route) =>
    route.fulfill({ json: { cards: [card], next_cursor: null } })
  );
  await page.goto('/discover');
  await page.getByRole('button', { name: 'Safety options for Priya' }).click();
  await expect(page.getByRole('heading', { name: 'Report Priya' })).toBeVisible();
  await page.getByRole('button', { name: 'Block instead' }).click();
  await expect(page.getByRole('heading', { name: 'Block Priya?' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Cancel' })).toBeFocused();
  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
});

test('recovery screen offers recover and continue', async ({ page }) => {
  await page.route('**/api/v1/account/recovery', (route) =>
    route.fulfill({ json: { recover_until: '2026-12-24T12:00:00Z' } })
  );
  await page.goto('/account/recovery');
  await expect(page.getByRole('heading', { name: /scheduled for deletion/i })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Recover my account' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Continue deleting' })).toBeVisible();
});

test('offline state is announced', async ({ page }) => {
  await page.goto('/');
  await page.evaluate(() => window.dispatchEvent(new Event('offline')));
  await expect(page.getByRole('status').filter({ hasText: 'You are offline' })).toBeVisible();
});

test('landing page has no Content-Security-Policy violations', async ({ page }) => {
  const violations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error' && /content security policy|refused to/i.test(message.text())) {
      violations.push(message.text());
    }
  });
  await page.goto('/');
  await expect(page.getByRole('heading', { name: /find a real game/i })).toBeVisible();
  expect(violations).toEqual([]);
});

test('landing page has no serious accessibility violations', async ({ page }) => {
  await page.goto('/');
  const results = await new AxeBuilder({ page }).analyze();
  const serious = results.violations.filter(
    (violation) => violation.impact === 'serious' || violation.impact === 'critical'
  );
  expect(serious).toEqual([]);
});
