import { expect, test, type Page } from '@playwright/test';

const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64'
);

const photo = (id: string, displayOrder: number, status: 'approved' | 'pending' | 'rejected') => ({
  photo_id: id,
  display_order: displayOrder,
  status
});

async function mockSignedIn(page: Page) {
  await page.route('**/api/v1/auth/session', (route) => route.fulfill({ status: 204, body: '' }));
  await page.route('**/api/v1/photos/*/url', (route) =>
    route.fulfill({ json: { url: 'data:image/png;base64,iVBORw0KGgo=' } })
  );
}

async function showPhotos(page: Page, photos: ReturnType<typeof photo>[]) {
  await page.route('**/api/v1/photos', (route) => route.fulfill({ json: { photos } }));
}

test('three photos replace the add control with the slot-limit helper', async ({ page }) => {
  await mockSignedIn(page);
  await showPhotos(page, [
    photo('11111111-1111-4111-8111-111111111111', 1, 'approved'),
    photo('22222222-2222-4222-8222-222222222222', 2, 'approved'),
    photo('33333333-3333-4333-8333-333333333333', 3, 'approved')
  ]);
  await page.goto('/profile/photos');

  await expect(page.getByText('You already have 3 photos.')).toBeVisible();
  await expect(page.getByText('Add a photo')).toHaveCount(0);
  await expect(page.getByText("Approved. You're in Discover.").first()).toBeVisible();
  await expect(page.locator('label[for="photo-upload-input"]')).toHaveCount(0);
});

test('an expired upload gets its own chip and copy', async ({ page }) => {
  await mockSignedIn(page);
  await showPhotos(page, [photo('11111111-1111-4111-8111-111111111111', 1, 'pending')]);
  await page.route('**/api/v1/photos/*/process', (route) =>
    route.fulfill({ status: 409, json: { error: 'photo cannot be processed' } })
  );
  await page.goto('/profile/photos');

  await expect(page.getByText('Not confirmed')).toBeVisible();
  await page.getByRole('button', { name: 'Check again' }).click();
  await expect(page.getByText('Expired', { exact: true })).toBeVisible();
  await expect(page.getByText('That upload took too long, so we cleared it. Add the photo again.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Try again' })).toBeVisible();
});

test('storage unavailable disables the add tile and offers retry', async ({ page }) => {
  await mockSignedIn(page);
  await showPhotos(page, []);
  await page.route('**/api/v1/photos/upload-url', (route) =>
    route.fulfill({ status: 503, json: { error: 'photo uploads unavailable' } })
  );
  await page.goto('/profile/photos');

  await page.locator('input[type="file"]').setInputFiles({
    name: 'face.png',
    mimeType: 'image/png',
    buffer: png
  });

  await expect(page.getByText("Photo uploads aren't available right now. Try again in a minute.").first()).toBeVisible();
  await expect(page.locator('p.btn[aria-disabled="true"]')).toHaveText('Add a photo');
  await expect(page.getByRole('button', { name: 'Retry' })).toBeVisible();
});

test('a failed check keeps the preview and offers a manual retry', async ({ page }) => {
  await mockSignedIn(page);
  await showPhotos(page, []);
  await page.route('**/api/v1/photos/upload-url', (route) =>
    route.fulfill({
      status: 201,
      json: {
        photo_id: '11111111-1111-4111-8111-111111111111',
        upload_url: 'http://127.0.0.1:4173/mock-upload'
      }
    })
  );
  await page.route('**/mock-upload', (route) => route.fulfill({ status: 200, body: '' }));
  await page.route('**/api/v1/photos/*/process', (route) =>
    route.fulfill({ status: 503, json: { error: 'photo checks unavailable' } })
  );
  await page.goto('/profile/photos');

  await page.locator('input[type="file"]').setInputFiles({
    name: 'face.png',
    mimeType: 'image/png',
    buffer: png
  });

  await expect(page.getByText('Check failed', { exact: true })).toBeVisible();
  await expect(page.getByText("We couldn't check this photo just now. Try again.")).toBeVisible();
  await expect(page.getByRole('button', { name: 'Try again' })).toBeVisible();
  await expect(page.getByAltText('Upload preview')).toBeVisible();
});

test('a rejected tile never requests or shows the image', async ({ page }) => {
  let imageRequests = 0;
  await mockSignedIn(page);
  await page.route('**/api/v1/photos/*/url', (route) => {
    imageRequests += 1;
    return route.fulfill({ status: 404, json: { error: 'photo not found' } });
  });
  await showPhotos(page, [photo('11111111-1111-4111-8111-111111111111', 1, 'rejected')]);
  await page.goto('/profile/photos');

  await expect(page.getByText('Not accepted', { exact: true })).toBeVisible();
  await expect(
    page.getByText(
      "This photo wasn't accepted. Photos need your face clearly visible, no other people, and no contact details."
    )
  ).toBeVisible();
  await expect(page.getByRole('button', { name: 'Upload another photo' })).toBeVisible();
  expect(imageRequests).toBe(0);
});

test('an unsupported file type is rejected locally with the iPhone hint', async ({ page }) => {
  await mockSignedIn(page);
  await showPhotos(page, []);
  let uploadRequests = 0;
  await page.route('**/api/v1/photos/upload-url', (route) => {
    uploadRequests += 1;
    return route.fulfill({ status: 503, json: { error: 'unexpected upload' } });
  });
  await page.goto('/profile/photos');

  await page.locator('input[type="file"]').setInputFiles({
    name: 'notes.txt',
    mimeType: 'text/plain',
    buffer: Buffer.from('not an image')
  });

  await expect(page.getByText('Wrong file type', { exact: true })).toBeVisible();
  await expect(page.getByText(/Use a JPEG, PNG or WebP photo/)).toBeVisible();
  await expect(page.getByText(/Most Compatible/)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Choose another' })).toBeVisible();
  expect(uploadRequests).toBe(0);
});
