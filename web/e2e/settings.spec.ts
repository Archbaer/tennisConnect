import { expect, test, type BrowserContext, type Page } from '@playwright/test';

import { tokens } from './auth';

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

async function headerLayout(page: Page) {
  return page.evaluate(() => {
    const column = document.querySelector('main .page')!.getBoundingClientRect();
    const heading = document.querySelector('main .page-head h1')!;
    const kicker = document.querySelector('main .page-head .kicker')!;
    const headingBox = heading.getBoundingClientRect();
    const kickerBox = kicker.getBoundingClientRect();
    return {
      textAlign: getComputedStyle(heading).textAlign,
      headingOffset: Math.round(headingBox.left - column.left),
      kickerOffset: Math.round(kickerBox.left - column.left),
      kickerAboveHeading: kickerBox.bottom <= headingBox.top
    };
  });
}

test('settings shows a read-only sign-in-only email row and no address', async ({ page, context }) => {
  await useSession(context);
  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible();

  const row = page.getByText('Email address — sign-in only', { exact: true });
  await expect(row).toBeVisible();
  await expect(page.getByRole('button', { name: /email address/i })).toHaveCount(0);
  await expect(page.getByRole('link', { name: /email address/i })).toHaveCount(0);

  const mainText = await page.locator('main').innerText();
  expect(mainText).not.toContain('@');
});

test('settings header uses the same page-head layout as the profile screen', async ({ page, context }) => {
  await useSession(context);
  await page.goto('/profile');
  await expect(page.getByRole('heading', { name: 'You', level: 1 })).toBeVisible();
  const profile = await headerLayout(page);

  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible();
  const settings = await headerLayout(page);

  expect(profile).toMatchObject({ textAlign: 'start', kickerAboveHeading: true });
  expect(settings).toMatchObject({ textAlign: 'start', kickerAboveHeading: true });
  expect(settings.headingOffset).toBe(profile.headingOffset);
  expect(settings.kickerOffset).toBe(profile.kickerOffset);
});
