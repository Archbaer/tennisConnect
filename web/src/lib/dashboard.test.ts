import { describe, expect, it } from 'vitest';
import type { Match } from './api';
import {
  activitySummary,
  buildDashboardMatches,
  buildKpis,
  chartGeometry,
  currentStreak,
  deriveResult,
  filterMatches,
  monthlyActivity,
  playWindows,
  type DashboardMatch
} from './dashboard';

function match(id: string, name: string, date: string): Match {
  return {
    match_id: id,
    player_id: 'player',
    display_name: name,
    matched_at: date,
    has_instagram: false,
    revealed: true,
    photo_urls: []
  };
}

function played(result: 'W' | 'L', date: string, hours = 1.5): DashboardMatch {
  return {
    id: 'M-TEST',
    matchId: 'test',
    opponent: 'Alex',
    score: '6–4, 6–3',
    venue: 'Battersea Park',
    result,
    playedAt: date,
    hours
  };
}

describe('deriveResult', () => {
  it('is stable for the same match id', () => {
    const first = deriveResult('22222222-2222-4222-8222-222222222222');
    expect(deriveResult('22222222-2222-4222-8222-222222222222')).toBe(first);
    expect(['W', 'L']).toContain(first);
  });
});

describe('buildDashboardMatches', () => {
  it('keeps real opponents and dates while deriving sample values', () => {
    const result = buildDashboardMatches([
      match('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', 'Priya', '2026-06-01T10:00:00Z'),
      match('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', 'Sam', '2026-06-10T10:00:00Z')
    ]);
    expect(result.map((row) => row.opponent)).toEqual(['Sam', 'Priya']);
    expect(result[0].id).toBe('M-BBBB');
    expect(result[0].score).toMatch(/^\d/);
    expect(result[0].venue.length).toBeGreaterThan(0);
    expect(['W', 'L']).toContain(result[0].result);
  });
});

describe('currentStreak', () => {
  it('counts consecutive wins from the most recent match', () => {
    expect(
      currentStreak([
        played('W', '2026-06-10T10:00:00Z'),
        played('W', '2026-06-05T10:00:00Z'),
        played('L', '2026-06-01T10:00:00Z')
      ])
    ).toBe(2);
  });
});

describe('buildKpis', () => {
  it('derives the four cards from match history', () => {
    const now = new Date('2026-06-15T12:00:00Z');
    const kpis = buildKpis(
      [
        played('W', '2026-06-10T10:00:00Z', 1.5),
        played('W', '2026-06-05T10:00:00Z', 1),
        played('L', '2026-05-20T10:00:00Z', 2)
      ],
      now
    );
    expect(kpis.map((kpi) => kpi.label)).toEqual([
      'Matches played',
      'Win rate',
      'Current streak',
      'Hours this month'
    ]);
    expect(kpis[0].value).toBe('3');
    expect(kpis[1].value).toBe('67%');
    expect(kpis[2].value).toBe('2');
    expect(kpis[3].value).toBe('2.5');
    expect(kpis[3].unit).toBe('h');
  });

  it('shows an empty state without matches', () => {
    const kpis = buildKpis([], new Date('2026-06-15T12:00:00Z'));
    expect(kpis[0].value).toBe('0');
    expect(kpis[1].value).toBe('—');
    expect(kpis[2].delta).toBe('No active streak');
  });
});

describe('monthlyActivity', () => {
  it('returns twelve ordered buckets and counts matches', () => {
    const monthly = monthlyActivity(
      [
        played('W', '2026-06-01T10:00:00Z'),
        played('W', '2026-06-12T10:00:00Z'),
        played('L', '2026-05-01T10:00:00Z'),
        played('W', '2025-08-01T10:00:00Z'),
        played('L', '2024-08-01T10:00:00Z')
      ],
      new Date('2026-06-15T12:00:00Z')
    );
    expect(monthly).toHaveLength(12);
    expect(monthly[0].label).toBe('Jul');
    expect(monthly[11].label).toBe('Jun');
    expect(monthly[11].count).toBe(2);
    expect(monthly[10].count).toBe(1);
    expect(monthly.reduce((sum, month) => sum + month.count, 0)).toBe(4);
    expect(activitySummary(monthly)).toContain('Busiest month: Jun with 2');
  });
});

describe('chartGeometry', () => {
  it('builds a closed area path and five gridlines', () => {
    const chart = chartGeometry([1, 2, 3]);
    expect(chart.points).toHaveLength(3);
    expect(chart.max).toBe(4);
    expect(chart.line.startsWith('M')).toBe(true);
    expect(chart.area.endsWith('Z')).toBe(true);
    expect(chart.ticks).toHaveLength(5);
    expect(chart.points[2].x).toBeGreaterThan(chart.points[0].x);
  });
});

describe('playWindows', () => {
  it('sums availability and labels the first three windows', () => {
    const windows = playWindows([
      { weekday: 2, start: '18:00', end: '21:00' },
      { weekday: 4, start: '18:00', end: '20:00' },
      { weekday: 1, start: '07:00', end: '08:00' }
    ]);
    expect(windows.totalHours).toBe(6);
    expect(windows.windows.map((window) => window.label)).toEqual([
      'Mon morning',
      'Tue evening',
      'Thu evening'
    ]);
    expect(windows.windows[1].percent).toBe(100);
  });
});

describe('filterMatches', () => {
  it('filters by opponent text and result', () => {
    const rows = [
      { ...played('W', '2026-06-10T10:00:00Z'), opponent: 'Priya', venue: 'Hyde Park' },
      { ...played('L', '2026-06-01T10:00:00Z'), opponent: 'Sam', venue: 'Battersea Park' }
    ];
    expect(filterMatches(rows, 'priya', 'all')).toHaveLength(1);
    expect(filterMatches(rows, 'park', 'L')).toHaveLength(1);
    expect(filterMatches(rows, '', 'W')).toHaveLength(1);
  });
});
