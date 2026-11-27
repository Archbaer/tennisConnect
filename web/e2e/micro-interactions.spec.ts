import { expect, test, type Page } from '@playwright/test';

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
    player_id: '11111111-1111-4111-8111-111111111111',
    display_name: 'Priya',
    matched_at: new Date(Date.now() - 54 * 60_000).toISOString(),
    has_instagram: false,
    revealed: true,
    photo_urls: []
  },
  {
    match_id: '33333333-3333-4333-8333-333333333333',
    player_id: '44444444-4444-4444-8444-444444444444',
    display_name: 'Sam',
    matched_at: new Date(Date.now() - 86400000).toISOString(),
    has_instagram: false,
    revealed: true,
    photo_urls: []
  }
];

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

async function mockSignedIn(page: Page) {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/matches', (route) => route.fulfill({ json: { matches } }));
  await page.route('**/api/v1/profile', (route) => route.fulfill({ json: profile }));
  await page.route('**/api/v1/discovery?**', (route) =>
    route.fulfill({ json: { cards: [card], next_cursor: null } })
  );
  await page.route('**/api/v1/photos', (route) => route.fulfill({ json: { photos: [] } }));
  await page.route('**/api/v1/location', (route) =>
    route.fulfill({ json: { area_label: 'Battersea' } })
  );
}

test('@mobile tab indicator moves between routes', async ({ page }) => {
  await mockSignedIn(page);
  await page.setViewportSize({ width: 360, height: 800 });
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: 'Overview', level: 1 })).toBeVisible();

  const tabBar = page.getByRole('navigation', { name: 'Primary mobile' });
  const indicator = page.locator('.tab-indicator');
  await expect(tabBar).toHaveAttribute('data-active', '0');
  await expect(tabBar.getByRole('link', { name: 'Dashboard' })).toHaveAttribute('aria-current', 'page');
  const before = await indicator.evaluate((element) => getComputedStyle(element).transform);

  await tabBar.getByRole('link', { name: 'Discover' }).click();
  await expect(page).toHaveURL(/\/discover$/);
  await expect(tabBar).toHaveAttribute('data-active', '1');
  await expect(tabBar.getByRole('link', { name: 'Discover' })).toHaveAttribute('aria-current', 'page');
  await expect(tabBar.getByRole('link', { name: 'Dashboard' })).not.toHaveAttribute('aria-current', 'page');

  await expect
    .poll(() => indicator.evaluate((element) => getComputedStyle(element).transform))
    .not.toBe(before);
});

test('@mobile match row swipe opens the match and keeps button and keyboard fallbacks', async ({ page }) => {
  await mockSignedIn(page);
  await page.route('**/api/v1/matches/22222222-2222-4222-8222-222222222222', (route) =>
    route.fulfill({ json: matches[0] })
  );
  await page.setViewportSize({ width: 390, height: 800 });
  await page.goto('/matches');
  await expect(page.getByRole('heading', { name: 'Matches', level: 1 })).toBeVisible();

  const row = page.locator('.matches-table tbody tr').first();
  const box = await row.boundingBox();
  expect(box).not.toBeNull();

  const pointer = (type: string, clientX: number, clientY: number) =>
    row.dispatchEvent(type, {
      pointerId: 1,
      pointerType: 'touch',
      isPrimary: true,
      button: 0,
      buttons: type === 'pointerup' ? 0 : 1,
      clientX,
      clientY
    });

  // A vertical drag must leave the row alone so the page can scroll.
  await pointer('pointerdown', box!.x + 200, box!.y + 30);
  await pointer('pointermove', box!.x + 204, box!.y + 140);
  await expect(row).not.toHaveClass(/is-swiping/);
  await pointer('pointerup', box!.x + 204, box!.y + 140);
  await expect(page).toHaveURL(/\/matches$/);

  // Mid-swipe the cue is revealed only in the gap left by the clipped content.
  await pointer('pointerdown', box!.x + 300, box!.y + 30);
  await pointer('pointermove', box!.x + 245, box!.y + 30);
  await expect(row).toHaveClass(/is-revealed/);

  const rowBox = await row.boundingBox();
  const cueBox = await row.locator('.swipe-cue').boundingBox();
  const contentBox = await row.locator('.match-actions').boundingBox();
  expect(rowBox).not.toBeNull();
  expect(cueBox).not.toBeNull();
  expect(contentBox).not.toBeNull();
  expect(cueBox!.width).toBeGreaterThan(40);
  expect(cueBox!.x).toBeGreaterThanOrEqual(contentBox!.x + contentBox!.width - 0.5);
  expect(cueBox!.x + cueBox!.width).toBeLessThanOrEqual(rowBox!.x + rowBox!.width + 1);
  expect(await row.evaluate((element) => getComputedStyle(element).overflow)).toBe('hidden');
  expect(
    await row.locator('.swipe-cue-label').evaluate((element) => Number(getComputedStyle(element).opacity))
  ).toBeGreaterThan(0);

  await pointer('pointermove', box!.x + 140, box!.y + 30);
  await expect(row).toHaveClass(/is-armed/);
  await pointer('pointerup', box!.x + 140, box!.y + 30);
  await expect(page).toHaveURL(/\/matches\/22222222-2222-4222-8222-222222222222$/);
  await expect(page.getByRole('heading', { name: 'Priya', level: 1 })).toBeVisible();

  // The explicit button works.
  await page.goto('/matches');
  await page.getByRole('link', { name: 'Open match with Priya' }).click();
  await expect(page).toHaveURL(/\/matches\/22222222-2222-4222-8222-222222222222$/);

  // Enter on the focused row link works.
  await page.goto('/matches');
  const person = page.locator('.match-person').first();
  await person.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/matches\/22222222-2222-4222-8222-222222222222$/);
});

test('skeletons stay busy until delayed data arrives', async ({ page }) => {
  let release: () => void = () => {};
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/matches', async (route) => {
    await gate;
    await route.fulfill({ json: { matches } });
  });

  await page.goto('/matches');
  const busy = page.locator('[aria-busy="true"]');
  await expect(busy).toBeVisible();
  await expect(busy.getByRole('status')).toHaveText('Loading…');
  await expect(busy.locator('.skeleton').first()).toBeVisible();

  release();
  await expect(page.getByRole('heading', { name: 'Total matches' })).toBeVisible();
  await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);
});

test('toast announces a sent match request and can be dismissed', async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/discovery?**', (route) =>
    route.fulfill({ json: { cards: [card], next_cursor: null } })
  );
  await page.route('**/api/v1/reactions', (route) => route.fulfill({ json: { matched: false } }));
  await page.goto('/discover');
  await expect(page.getByRole('heading', { name: 'Priya, 32' })).toBeVisible();

  await page.getByRole('button', { name: "I'd play" }).click();

  const toaster = page.locator('.toaster');
  await expect(toaster).toHaveAttribute('role', 'status');
  await expect(toaster).toHaveAttribute('aria-live', 'polite');
  await expect(toaster.getByText('Match request sent.')).toBeVisible();

  await toaster.getByRole('button', { name: 'Dismiss notification' }).click();
  await expect(toaster.getByText('Match request sent.')).toHaveCount(0);
});

test('toast confirms a saved profile', async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/profile', (route) =>
    route.request().method() === 'PUT' ? route.fulfill({ status: 204, body: '' }) : route.fulfill({ json: profile })
  );
  await page.route('**/api/v1/photos', (route) => route.fulfill({ json: { photos: [] } }));
  await page.route('**/api/v1/location', (route) =>
    route.fulfill({ json: { area_label: 'Battersea' } })
  );
  await page.goto('/profile/edit');
  await expect(page.getByRole('heading', { name: 'Edit profile' })).toBeVisible();

  await page.getByLabel('Display name').fill('E2E Player Two');
  await page.getByRole('button', { name: 'Save changes' }).click();

  await expect(page).toHaveURL(/\/profile$/);
  const toaster = page.locator('.toaster');
  await expect(toaster.getByText('Profile saved.')).toBeVisible();
  await toaster.getByRole('button', { name: 'Dismiss notification' }).click();
  await expect(toaster.getByText('Profile saved.')).toHaveCount(0);
});

test('@mobile reduced motion disables the indicator transition and skeleton shimmer', async ({ page }) => {
  let release: () => void = () => {};
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/matches', async (route) => {
    await gate;
    await route.fulfill({ json: { matches } });
  });
  await page.route('**/api/v1/profile', (route) => route.fulfill({ json: profile }));
  await page.setViewportSize({ width: 390, height: 800 });
  await page.goto('/dashboard');

  const indicator = page.locator('.tab-indicator');
  await expect(indicator).toBeVisible();
  expect(await indicator.evaluate((element) => getComputedStyle(element).transitionDuration)).toBe('0s');

  const skeleton = page.locator('.skeleton').first();
  await expect(skeleton).toBeVisible();
  expect(await skeleton.evaluate((element) => getComputedStyle(element, '::after').animationName)).toBe('none');

  release();
  await expect(page.getByRole('heading', { name: 'Overview', level: 1 })).toBeVisible();
});
