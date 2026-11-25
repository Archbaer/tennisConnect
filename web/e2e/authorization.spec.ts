import { expect, test, type BrowserContext } from '@playwright/test';

import { tokens } from './auth';

// Tokens are generated per run and hashed into e2e/seed.sql.

async function useSession(context: BrowserContext, token: string) {
  await context.addCookies([
    {
      name: 'tennisconnect_session',
      value: token,
      url: 'http://127.0.0.1:4173',
      httpOnly: true,
      sameSite: 'Lax'
    }
  ]);
}

async function useReviewerSession(context: BrowserContext) {
  await context.addCookies([
    {
      name: 'tennisconnect_reviewer',
      value: tokens().reviewer,
      url: 'http://127.0.0.1:4173',
      httpOnly: true,
      sameSite: 'Strict'
    }
  ]);
}

test('signed-out browser is sent away from a player screen', async ({ page }) => {
  await page.goto('/discover');
  await expect(page).toHaveURL(/\?next=%2Fdiscover$/);
  await expect(page.getByRole('heading', { name: /find a real game/i })).toBeVisible();
});

test('player session reads discovery from the real API', async ({ page, context }) => {
  await useSession(context, tokens().player);
  await page.goto('/discover');
  await expect(page.getByRole('heading', { name: 'Players near you' })).toBeVisible();
  await expect(page.getByRole('heading', { name: /Navya/ })).toBeVisible();
});

test('banned session is limited to the appeal screen', async ({ page, context }) => {
  await useSession(context, tokens().banned);
  await page.goto('/discover');
  await expect(page).toHaveURL('/appeal');
  await expect(page.getByRole('heading', { name: /account is suspended/i })).toBeVisible();
  await expect(page.getByText(/one appeal|appeal window/i).first()).toBeVisible();
});

test('player cannot open the reviewer queue', async ({ page, context }) => {
  await useSession(context, tokens().player);
  await page.goto('/reviewer');
  await expect(page.getByRole('heading', { name: 'Reviewer sign in' })).toBeVisible();
});

test('reviewer session without MFA cannot read the queue', async ({ page, context }) => {
  await useReviewerSession(context);
  await page.goto('/reviewer');
  await expect(page.getByRole('heading', { name: 'Second factor' })).toBeVisible();
});

test('settings opens area without entering onboarding', async ({ page, context }) => {
  await useSession(context, tokens().player);
  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Area and discovery radius' })).toBeVisible();
  await page.waitForTimeout(500);
  await page.getByRole('button', { name: 'Area and discovery radius' }).click();
  await expect(page.getByRole('heading', { name: 'Area and distance' })).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText('Battersea').first()).toBeVisible();
  await expect(page).toHaveURL('/settings');
});

test('profile preview opens the player card', async ({ page, context }) => {
  await useSession(context, tokens().player);
  await page.goto('/profile');
  await page.getByRole('button', { name: 'Preview as another player' }).click();
  await expect(page.getByRole('heading', { name: 'Your card' })).toBeVisible();
  await expect(page.getByRole('dialog').locator('h3')).toContainText('E2E Player');
});

test('denied location permission falls back to manual area', async ({ page, context }) => {
  await useSession(context, tokens().player);
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'geolocation', {
      configurable: true,
      value: {
        getCurrentPosition: (_success: PositionCallback, failure?: PositionErrorCallback) =>
          failure?.({
            code: 1,
            message: 'Permission denied',
            PERMISSION_DENIED: 1,
            POSITION_UNAVAILABLE: 2,
            TIMEOUT: 3
          })
      }
    });
  });
  await page.goto('/onboarding');
  await expect(page.getByRole('heading', { name: 'Where you play' })).toBeVisible();
  await page.getByRole('button', { name: 'Use my location' }).click();
  await expect(page.getByText(/Location permission was denied/i)).toBeVisible();
  await expect(page.getByLabel(/Postcode or area/i)).toBeVisible();
});
