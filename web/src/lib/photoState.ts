import type { Photo } from './api';

export type PhotoStatus = Photo['status'];

export type PhotoAction = 'check_again' | 'try_again' | 'choose_another' | 'upload_another' | 'retry';

export type PhotoStatusCopy = {
  chip: string;
  message: string;
  action: PhotoAction | null;
};

export type UploadFailure =
  | 'upload_failed'
  | 'check_failed'
  | 'expired'
  | 'too_large'
  | 'wrong_type'
  | 'unreadable'
  | 'storage_unavailable';

export type UploadFailureDetail = {
  megabytes?: number;
  megapixels?: number;
};

export type UploadFailureCopy = {
  chip: string;
  message: string;
  action: PhotoAction;
};

export const slotLimitCopy = 'You already have 3 photos.';

export function photoStatusCopy(status: PhotoStatus): PhotoStatusCopy {
  switch (status) {
    case 'approved':
      return { chip: 'Approved', message: "Approved. You're in Discover.", action: null };
    case 'rejected':
      return {
        chip: 'Not accepted',
        message:
          "This photo wasn't accepted. Photos need your face clearly visible, no other people, and no contact details.",
        action: 'upload_another'
      };
    default:
      return {
        chip: 'Not confirmed',
        message: "We couldn't confirm this photo is ready yet. It isn't visible to anyone.",
        action: 'check_again'
      };
  }
}

export function uploadFailureCopy(
  failure: UploadFailure,
  detail: UploadFailureDetail = {}
): UploadFailureCopy {
  switch (failure) {
    case 'check_failed':
      return {
        chip: 'Check failed',
        message: "We couldn't check this photo just now. Try again.",
        action: 'try_again'
      };
    case 'expired':
      return {
        chip: 'Expired',
        message: 'That upload took too long, so we cleared it. Add the photo again.',
        action: 'try_again'
      };
    case 'too_large':
      return {
        chip: 'Too large',
        message: detail.megapixels
          ? `That photo is ${detail.megapixels.toFixed(0)} megapixels. The limit is 20. Choose a smaller version.`
          : `That photo is ${(detail.megabytes ?? 0).toFixed(1)} MB. The limit is 5 MB. Choose a smaller one.`,
        action: 'choose_another'
      };
    case 'wrong_type':
      return {
        chip: 'Wrong file type',
        message:
          'We can’t read that file. Use a JPEG, PNG or WebP photo. On iPhone, Settings › Camera › Formats › Most Compatible.',
        action: 'choose_another'
      };
    case 'unreadable':
      return {
        chip: 'Upload failed',
        message: 'We couldn’t read that image. Choose a different file.',
        action: 'choose_another'
      };
    case 'storage_unavailable':
      return {
        chip: 'Uploads unavailable',
        message: "Photo uploads aren't available right now. Try again in a minute.",
        action: 'retry'
      };
    default:
      return {
        chip: 'Upload failed',
        message: "The upload didn't finish. Check your connection and try again.",
        action: 'try_again'
      };
  }
}

export function addControlText(photoCount: number, busy: boolean, stage: string): string {
  if (busy) return stage || 'Working…';
  if (photoCount >= 3) return slotLimitCopy;
  return 'Add a photo';
}
