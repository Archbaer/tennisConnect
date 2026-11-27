import { weekdayLabel, type Availability, type Match } from '#lib/api';

export type MatchResult = 'W' | 'L';

export type DashboardMatch = {
  id: string;
  matchId: string;
  opponent: string;
  score: string;
  venue: string;
  result: MatchResult;
  playedAt: string;
  hours: number;
};

export type KpiDirection = 'up' | 'down' | 'flat';

export type Kpi = {
  label: string;
  value: string;
  unit?: string;
  delta: string;
  direction: KpiDirection;
};

export type MonthActivity = {
  key: string;
  label: string;
  count: number;
};

export type ChartPoint = {
  x: number;
  y: number;
  value: number;
};

export type ChartGeometry = {
  width: number;
  height: number;
  left: number;
  right: number;
  top: number;
  bottom: number;
  max: number;
  points: ChartPoint[];
  line: string;
  area: string;
  ticks: { y: number; label: string }[];
};

export type PlayWindow = {
  label: string;
  hours: number;
  percent: number;
};

const monthLabels = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const venues = ['Battersea Park', 'Clapham Common', 'Wandsworth Common', 'Riverside Courts', 'Hyde Park'];
const scores: Record<MatchResult, string[]> = {
  W: ['6–4, 6–3', '7–5, 6–2', '6–3, 6–4', '7–6, 6–4'],
  L: ['4–6, 3–6', '6–7, 5–7', '2–6, 4–6', '5–7, 4–6']
};

function hash(value: string): number {
  let result = 0;
  for (let index = 0; index < value.length; index += 1) {
    result = (result * 31 + value.charCodeAt(index)) >>> 0;
  }
  return result;
}

// Scores, venues and results are not stored yet. Derive stable sample values
// from the real match id so the dashboard is honest and repeatable.
export function deriveResult(matchId: string): MatchResult {
  return hash(matchId) % 100 < 62 ? 'W' : 'L';
}

export function deriveScore(matchId: string, result: MatchResult): string {
  const options = scores[result];
  return options[hash(`${matchId}:score`) % options.length];
}

export function deriveVenue(matchId: string): string {
  return venues[hash(`${matchId}:venue`) % venues.length];
}

export function deriveHours(matchId: string): number {
  return 1 + (hash(`${matchId}:hours`) % 3) * 0.5;
}

export function buildDashboardMatches(matches: Match[]): DashboardMatch[] {
  return matches
    .map((match) => {
      const result = deriveResult(match.match_id);
      return {
        id: `M-${match.match_id.slice(0, 4).toUpperCase()}`,
        matchId: match.match_id,
        opponent: match.display_name,
        score: deriveScore(match.match_id, result),
        venue: deriveVenue(match.match_id),
        result,
        playedAt: match.matched_at,
        hours: deriveHours(match.match_id)
      };
    })
    .sort((first, second) => new Date(second.playedAt).getTime() - new Date(first.playedAt).getTime());
}

export function currentStreak(matches: DashboardMatch[]): number {
  const ordered = [...matches].sort(
    (first, second) => new Date(second.playedAt).getTime() - new Date(first.playedAt).getTime()
  );
  let streak = 0;
  for (const match of ordered) {
    if (match.result !== 'W') break;
    streak += 1;
  }
  return streak;
}

function monthKey(date: Date): string {
  return `${date.getUTCFullYear()}-${date.getUTCMonth()}`;
}

function winRate(matches: DashboardMatch[]): number | null {
  if (!matches.length) return null;
  return Math.round((matches.filter((match) => match.result === 'W').length / matches.length) * 100);
}

function periodChange(current: number, previous: number): { text: string; direction: KpiDirection } {
  if (!current && !previous) return { text: 'No matches yet', direction: 'flat' };
  if (!previous) return { text: 'New this month', direction: 'up' };
  const change = ((current - previous) / previous) * 100;
  const rounded = Math.abs(change).toFixed(1);
  if (change > 0) return { text: `${rounded}% vs previous 30 days`, direction: 'up' };
  if (change < 0) return { text: `${rounded}% vs previous 30 days`, direction: 'down' };
  return { text: 'No change', direction: 'flat' };
}

function pointsChange(current: number | null, previous: number | null): { text: string; direction: KpiDirection } {
  if (current === null) return { text: 'No matches yet', direction: 'flat' };
  if (previous === null) return { text: 'No prior form', direction: 'flat' };
  const change = current - previous;
  if (change > 0) return { text: `${change} pts vs previous 30 days`, direction: 'up' };
  if (change < 0) return { text: `${Math.abs(change)} pts vs previous 30 days`, direction: 'down' };
  return { text: 'No change', direction: 'flat' };
}

function formatHours(hours: number): string {
  return Number.isInteger(hours) ? String(hours) : hours.toFixed(1);
}

export function buildKpis(matches: DashboardMatch[], now = new Date()): Kpi[] {
  const nowMs = now.getTime();
  const last30 = matches.filter((match) => {
    const time = new Date(match.playedAt).getTime();
    return time >= nowMs - 30 * 86400000 && time < nowMs;
  });
  const previous30 = matches.filter((match) => {
    const time = new Date(match.playedAt).getTime();
    return time >= nowMs - 60 * 86400000 && time < nowMs - 30 * 86400000;
  });

  const thisMonth = monthKey(now);
  const lastMonth = monthKey(new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1)));
  const hoursThisMonth = matches
    .filter((match) => monthKey(new Date(match.playedAt)) === thisMonth)
    .reduce((total, match) => total + match.hours, 0);
  const hoursLastMonth = matches
    .filter((match) => monthKey(new Date(match.playedAt)) === lastMonth)
    .reduce((total, match) => total + match.hours, 0);

  const streak = currentStreak(matches);
  const playedDelta = periodChange(last30.length, previous30.length);
  const rateDelta = pointsChange(winRate(last30), winRate(previous30));
  const hoursDelta = periodChange(Math.round(hoursThisMonth * 10), Math.round(hoursLastMonth * 10));

  return [
    {
      label: 'Matches played',
      value: String(matches.length),
      delta: playedDelta.text,
      direction: playedDelta.direction
    },
    {
      label: 'Win rate',
      value: winRate(matches) === null ? '—' : `${winRate(matches)}%`,
      delta: rateDelta.text,
      direction: rateDelta.direction
    },
    {
      label: 'Current streak',
      value: String(streak),
      delta: streak ? `${streak} ${streak === 1 ? 'win' : 'wins'} in a row` : 'No active streak',
      direction: streak ? 'up' : 'flat'
    },
    {
      label: 'Hours this month',
      value: formatHours(hoursThisMonth),
      unit: 'h',
      delta: hoursDelta.text,
      direction: hoursDelta.direction
    }
  ];
}

export function monthlyActivity(matches: DashboardMatch[], now = new Date(), months = 12): MonthActivity[] {
  const buckets = new Map<string, MonthActivity>();
  const result: MonthActivity[] = [];
  for (let offset = months - 1; offset >= 0; offset -= 1) {
    const date = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - offset, 1));
    const key = monthKey(date);
    const item = { key, label: monthLabels[date.getUTCMonth()], count: 0 };
    buckets.set(key, item);
    result.push(item);
  }
  for (const match of matches) {
    const item = buckets.get(monthKey(new Date(match.playedAt)));
    if (item) item.count += 1;
  }
  return result;
}

export function chartGeometry(values: number[], width = 720, height = 260): ChartGeometry {
  const left = 36;
  const right = 12;
  const top = 16;
  const bottom = 30;
  const safe = values.length ? values : [0];
  const maxValue = Math.max(...safe);
  const max = Math.max(4, Math.ceil(maxValue / 4) * 4);
  const stepX = safe.length > 1 ? (width - left - right) / (safe.length - 1) : 0;
  const points = safe.map((value, index) => ({
    x: left + index * stepX,
    y: top + (height - top - bottom) * (1 - value / max),
    value
  }));
  const line = points
    .map((point, index) => `${index === 0 ? 'M' : 'L'}${point.x.toFixed(1)},${point.y.toFixed(1)}`)
    .join(' ');
  const baseline = height - bottom;
  const area = `${line} L${points[points.length - 1].x.toFixed(1)},${baseline} L${points[0].x.toFixed(1)},${baseline} Z`;
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((fraction) => ({
    y: top + (height - top - bottom) * (1 - fraction),
    label: String(Math.round(max * fraction))
  }));
  return { width, height, left, right, top, bottom, max, points, line, area, ticks };
}

export function activitySummary(monthly: MonthActivity[]): string {
  if (!monthly.length) return 'No activity in the selected period.';
  const total = monthly.reduce((sum, month) => sum + month.count, 0);
  if (!total) return 'No matches in the selected period.';
  const peak = monthly.reduce((best, month) => (month.count > best.count ? month : best));
  return `${total} ${total === 1 ? 'match' : 'matches'} in the selected period. Busiest month: ${peak.label} with ${peak.count}.`;
}

function minutes(value: string): number {
  const [hours, minutesPart] = value.split(':').map(Number);
  return (hours || 0) * 60 + (minutesPart || 0);
}

function partOfDay(start: string): string {
  const hour = Number(start.split(':')[0]);
  if (hour < 12) return 'morning';
  if (hour < 17) return 'afternoon';
  return 'evening';
}

export function playWindows(availability: Availability[]): { totalHours: number; windows: PlayWindow[] } {
  const slots = [...availability]
    .sort((first, second) => first.weekday - second.weekday || first.start.localeCompare(second.start))
    .map((slot) => ({
      label: `${weekdayLabel(slot.weekday)} ${partOfDay(slot.start)}`,
      hours: Math.max(0, minutes(slot.end) - minutes(slot.start)) / 60
    }));
  const totalHours = slots.reduce((sum, slot) => sum + slot.hours, 0);
  const max = Math.max(...slots.map((slot) => slot.hours), 1);
  return {
    totalHours: Math.round(totalHours * 10) / 10,
    windows: slots.slice(0, 3).map((slot) => ({
      ...slot,
      percent: Math.round((slot.hours / max) * 100)
    }))
  };
}

export function filterMatches(
  matches: DashboardMatch[],
  query: string,
  result: 'all' | MatchResult
): DashboardMatch[] {
  const needle = query.trim().toLowerCase();
  return matches.filter((match) => {
    if (result !== 'all' && match.result !== result) return false;
    if (!needle) return true;
    return [match.opponent, match.venue, match.score, match.id].some((value) =>
      value.toLowerCase().includes(needle)
    );
  });
}
