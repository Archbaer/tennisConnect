import { expect, test, type Page } from '@playwright/test';

const player = {
  player_id: '11111111-1111-4111-8111-111111111111',
  display_name: 'Priya',
  age: 32,
  gender: 'woman',
  skill_level: 'intermediate',
  bio: 'Tuesday evening rallies.',
  play_summary: 'Tue, Thu evenings',
  photo_urls: [],
  distance_miles: 3
};

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

const matches = [
  {
    match_id: '22222222-2222-4222-8222-222222222222',
    player_id: player.player_id,
    display_name: 'Priya',
    matched_at: new Date().toISOString(),
    has_instagram: false,
    revealed: true,
    photo_urls: []
  },
  {
    match_id: '33333333-3333-4333-8333-333333333333',
    player_id: '44444444-4444-4444-8444-444444444444',
    display_name: 'Sam',
    matched_at: new Date(Date.now() - 86400000).toISOString(),
    has_instagram: true,
    revealed: false,
    photo_urls: []
  }
];

async function mockSignedIn(page: Page) {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/matches', (route) => route.fulfill({ json: { matches } }));
  await page.route('**/api/v1/profile', (route) => route.fulfill({ json: profile }));
  await page.route('**/api/v1/discovery?**', (route) =>
    route.fulfill({ json: { cards: [{ player }], next_cursor: null } })
  );
  await page.route('**/api/v1/photos', (route) => route.fulfill({ json: { photos: [] } }));
  await page.route('**/api/v1/location', (route) =>
    route.fulfill({ json: { area_label: 'Battersea' } })
  );
}

const screens = [
  { name: 'landing', path: '/', signedIn: false, heading: /find a real game/i },
  { name: 'dashboard', path: '/dashboard', signedIn: true, heading: 'Overview' },
  { name: 'discover', path: '/discover', signedIn: true, heading: 'Players near you' },
  { name: 'matches', path: '/matches', signedIn: true, heading: 'Matches' },
  { name: 'profile', path: '/profile', signedIn: true, heading: 'You' },
  { name: 'settings', path: '/settings', signedIn: true, heading: 'Settings' }
];

for (const screen of screens) {
  test(`@mobile ${screen.name} has no horizontal overflow at 360px and 390px`, async ({ page }) => {
    if (screen.signedIn) {
      await mockSignedIn(page);
    } else {
      await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 401, body: '' }));
    }

    for (const width of [360, 390]) {
      await page.setViewportSize({ width, height: 800 });
      await page.goto(screen.path);
      await expect(page.getByRole('heading', { name: screen.heading }).first()).toBeVisible();

      const overflow = await page.evaluate(
        () =>
          Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) > window.innerWidth
      );
      expect(overflow, `${screen.name} at ${width}px`).toBe(false);
    }
  });
}

test('@mobile bottom tab bar has five thumb-friendly targets', async ({ page }) => {
  await mockSignedIn(page);
  await page.setViewportSize({ width: 360, height: 800 });
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible();

  const tabBar = page.getByRole('navigation', { name: 'Primary mobile' });
  await expect(tabBar).toBeVisible();
  const links = tabBar.getByRole('link');
  await expect(links).toHaveCount(5);

  for (const name of ['Dashboard', 'Discover', 'Matches', 'You', 'Settings']) {
    const box = await tabBar.getByRole('link', { name: new RegExp(`^${name}`) }).boundingBox();
    expect(box, `${name} tab`).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);
    expect(box!.width).toBeGreaterThanOrEqual(44);
  }
});

test('@mobile tables reflow into labelled stacked cards', async ({ page }) => {
  await mockSignedIn(page);
  await page.setViewportSize({ width: 390, height: 800 });

  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible();
  await expect(page.getByRole('table')).toBeVisible();
  const dashboard = await page.locator('.dashboard-table').evaluate((table) => {
    const row = table.querySelector('tbody tr');
    const cell = table.querySelector('tbody td[data-label="Opponent"]');
    return {
      table: getComputedStyle(table).display,
      row: row ? getComputedStyle(row).display : '',
      label: cell ? getComputedStyle(cell, '::before').content : ''
    };
  });
  expect(dashboard.table).toBe('block');
  expect(dashboard.row).toBe('block');
  expect(dashboard.label).toContain('Opponent');

  await page.goto('/matches');
  await expect(page.getByRole('heading', { name: 'Matches', level: 1 })).toBeVisible();
  const matchRow = await page.locator('.matches-table').evaluate((table) => {
    const row = table.querySelector('tbody tr');
    const cell = table.querySelector('tbody td[data-label="Player"]');
    return {
      row: row ? getComputedStyle(row).display : '',
      label: cell ? getComputedStyle(cell, '::before').content : ''
    };
  });
  expect(matchRow.row).toBe('block');
  expect(matchRow.label).toContain('Player');
});

test('@mobile collapsible feature cards toggle open and closed', async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 401, body: '' }));
  await page.setViewportSize({ width: 360, height: 800 });
  await page.goto('/');
  await expect(page.getByRole('heading', { name: /find a real game/i })).toBeVisible();

  const feature = page.locator('details').filter({ hasText: 'Photos checked first' }).first();
  const body = page.getByText('Moderation runs before visibility.');
  expect(await feature.evaluate((element) => (element as HTMLDetailsElement).open)).toBe(false);
  await expect(body).toBeHidden();

  await feature.locator('summary').click();
  expect(await feature.evaluate((element) => (element as HTMLDetailsElement).open)).toBe(true);
  await expect(body).toBeVisible();

  await feature.locator('summary').click();
  expect(await feature.evaluate((element) => (element as HTMLDetailsElement).open)).toBe(false);
  await expect(body).toBeHidden();
});
