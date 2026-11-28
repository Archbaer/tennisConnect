import { describe, expect, it } from 'vitest';
import {
  addControlText,
  photoStatusCopy,
  slotLimitCopy,
  uploadFailureCopy
} from './photoState';

describe('photoStatusCopy', () => {
  it('maps the three server statuses to the spec chips and sentences', () => {
    expect(photoStatusCopy('approved')).toEqual({
      chip: 'Approved',
      message: "Approved. You're in Discover.",
      action: null
    });
    expect(photoStatusCopy('pending')).toEqual({
      chip: 'Not confirmed',
      message: "We couldn't confirm this photo is ready yet. It isn't visible to anyone.",
      action: 'check_again'
    });
    expect(photoStatusCopy('rejected')).toEqual({
      chip: 'Not accepted',
      message:
        "This photo wasn't accepted. Photos need your face clearly visible, no other people, and no contact details.",
      action: 'upload_another'
    });
  });

  it('never names a provider or a rejection cause', () => {
    const copy = JSON.stringify(photoStatusCopy('rejected'));
    for (const banned of ['nudity', 'score', 'threshold', 'classifier', 'provider']) {
      expect(copy.toLowerCase()).not.toContain(banned);
    }
  });
});

describe('uploadFailureCopy', () => {
  it('uses the check-failed state with a manual retry', () => {
    expect(uploadFailureCopy('check_failed')).toEqual({
      chip: 'Check failed',
      message: "We couldn't check this photo just now. Try again.",
      action: 'try_again'
    });
  });

  it('gives an expired upload its own state', () => {
    expect(uploadFailureCopy('expired')).toEqual({
      chip: 'Expired',
      message: 'That upload took too long, so we cleared it. Add the photo again.',
      action: 'try_again'
    });
  });

  it('reports the local size and pixel limits without claiming a server cause', () => {
    expect(uploadFailureCopy('too_large', { megabytes: 12.4 }).message).toBe(
      'That photo is 12.4 MB. The limit is 5 MB. Choose a smaller one.'
    );
    expect(uploadFailureCopy('too_large', { megapixels: 34 }).message).toBe(
      'That photo is 34 megapixels. The limit is 20. Choose a smaller version.'
    );
    expect(uploadFailureCopy('too_large', { megapixels: 34 }).chip).toBe('Too large');
  });

  it('includes the iPhone hint for an unsupported file type', () => {
    const copy = uploadFailureCopy('wrong_type');
    expect(copy.chip).toBe('Wrong file type');
    expect(copy.message).toContain('Use a JPEG, PNG or WebP photo.');
    expect(copy.message).toContain('Most Compatible');
  });

  it('keeps storage unavailable retryable and the add control disabled', () => {
    expect(uploadFailureCopy('storage_unavailable')).toEqual({
      chip: 'Uploads unavailable',
      message: "Photo uploads aren't available right now. Try again in a minute.",
      action: 'retry'
    });
  });

  it('falls back to the upload-failed sentence', () => {
    expect(uploadFailureCopy('upload_failed')).toEqual({
      chip: 'Upload failed',
      message: "The upload didn't finish. Check your connection and try again.",
      action: 'try_again'
    });
  });
});

describe('slot limit', () => {
  it('replaces the add control with the helper text at three photos', () => {
    expect(slotLimitCopy).toBe('You already have 3 photos.');
    expect(addControlText(2, false, '')).toBe('Add a photo');
    expect(addControlText(3, false, '')).toBe(slotLimitCopy);
  });

  it('uses the active stage while busy', () => {
    expect(addControlText(0, true, 'Uploading 62%')).toBe('Uploading 62%');
    expect(addControlText(0, true, '')).toBe('Working…');
  });
});
