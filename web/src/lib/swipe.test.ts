import { describe, expect, it } from 'vitest';
import {
  SWIPE_MAX_PX,
  SWIPE_OPEN_PX,
  SWIPE_REVEAL_PX,
  swipeAxis,
  swipeOffset,
  swipeOutcome
} from './swipe';

describe('swipeAxis', () => {
  it('waits for the slop distance', () => {
    expect(swipeAxis(4, 4)).toBe('none');
  });

  it('locks to the dominant axis', () => {
    expect(swipeAxis(30, 6)).toBe('horizontal');
    expect(swipeAxis(-30, 6)).toBe('horizontal');
    expect(swipeAxis(6, 30)).toBe('vertical');
    expect(swipeAxis(0, 80)).toBe('vertical');
  });
});

describe('swipeOutcome', () => {
  it('resets below the reveal threshold', () => {
    expect(swipeOutcome(0)).toBe('reset');
    expect(swipeOutcome(SWIPE_REVEAL_PX - 1)).toBe('reset');
  });

  it('reveals at the reveal threshold', () => {
    expect(swipeOutcome(SWIPE_REVEAL_PX)).toBe('reveal');
    expect(swipeOutcome(SWIPE_OPEN_PX - 1)).toBe('reveal');
  });

  it('opens at the open threshold in either direction', () => {
    expect(swipeOutcome(SWIPE_OPEN_PX)).toBe('open');
    expect(swipeOutcome(-SWIPE_OPEN_PX)).toBe('open');
  });
});

describe('swipeOffset', () => {
  it('follows the pointer before the reveal width', () => {
    expect(swipeOffset(50)).toBe(50);
    expect(swipeOffset(-50)).toBe(-50);
  });

  it('adds resistance and clamps far drags', () => {
    expect(swipeOffset(SWIPE_MAX_PX + 10)).toBeCloseTo(SWIPE_MAX_PX + 3);
    expect(swipeOffset(500)).toBe(SWIPE_MAX_PX + 24);
    expect(swipeOffset(-500)).toBe(-(SWIPE_MAX_PX + 24));
  });
});
