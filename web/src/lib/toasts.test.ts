import { get } from 'svelte/store';
import { beforeEach, describe, expect, it } from 'vitest';
import { clearToasts, dismissToast, MAX_TOASTS, showToast, toasts } from './toasts';

beforeEach(() => {
  clearToasts();
});

describe('toast store', () => {
  it('adds an announced toast by default', () => {
    const id = showToast('Profile saved.');

    expect(get(toasts)).toEqual([{ id, message: 'Profile saved.', announce: true }]);
  });

  it('can keep a toast visual only', () => {
    showToast('Match request sent.', { announce: false });

    expect(get(toasts)[0].announce).toBe(false);
  });

  it('dismisses a toast by id', () => {
    const first = showToast('First');
    const second = showToast('Second');

    dismissToast(first);

    expect(get(toasts).map((toast) => toast.id)).toEqual([second]);
  });

  it('keeps only the newest toasts in the stack', () => {
    for (let index = 0; index < MAX_TOASTS + 2; index++) {
      showToast(`Toast ${index}`);
    }

    expect(get(toasts).map((toast) => toast.message)).toEqual(['Toast 2', 'Toast 3', 'Toast 4']);
  });
});
