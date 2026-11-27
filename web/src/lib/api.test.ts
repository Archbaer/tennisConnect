import { describe, expect, it } from 'vitest';
import { distanceLabel, reportCategories, weekdayLabel, weekdayName } from './api';

describe('distanceLabel', () => {
  it('uses the under-one-mile copy for zero', () => {
    expect(distanceLabel(0)).toBe('Under 1 mile');
  });

  it('uses singular and plural whole miles', () => {
    expect(distanceLabel(1)).toBe('1 mile');
    expect(distanceLabel(3)).toBe('3 miles');
  });
});

describe('weekdayLabel', () => {
  it('uses ISO weekday numbering', () => {
    expect(weekdayLabel(1)).toBe('Mon');
    expect(weekdayLabel(7)).toBe('Sun');
  });

  it('provides full weekday names for text lists', () => {
    expect(weekdayName(1)).toBe('Monday');
    expect(weekdayName(2)).toBe('Tuesday');
    expect(weekdayName(7)).toBe('Sunday');
  });
});

describe('report categories', () => {
  it('matches the backend enum', () => {
    expect(reportCategories.map(([value]) => value)).toEqual([
      'fake_profile',
      'inappropriate_photos',
      'harassment',
      'contact_details',
      'underage',
      'other'
    ]);
  });
});
