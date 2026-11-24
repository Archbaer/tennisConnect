export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

const apiBase = import.meta.env.PUBLIC_API_BASE_URL ?? '';

export async function api(path: string, init: RequestInit = {}): Promise<Response> {
  if (typeof navigator !== 'undefined' && !navigator.onLine) {
    throw new ApiError(0, 'You are offline. Reconnect and try again.');
  }
  let response: Response;
  try {
    response = await fetch(`${apiBase}${path}`, {
      ...init,
      credentials: 'include',
      headers: {
        ...(init.body ? { 'Content-Type': 'application/json' } : {}),
        ...init.headers
      }
    });
  } catch (problem) {
    if (problem instanceof TypeError) {
      throw new ApiError(0, 'You appear to be offline. Reconnect and try again.');
    }
    throw problem;
  }
  if (!response.ok) {
    let message = 'Something went wrong.';
    try {
      const body = await response.json();
      message = body.error ?? message;
    } catch {
      // Keep the generic message for non-JSON failures.
    }
    throw new ApiError(response.status, message);
  }
  return response;
}

export async function apiJSON<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await api(path, init);
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export type Availability = { weekday: number; start: string; end: string };

export type Profile = {
  display_name: string;
  gender: 'woman' | 'man';
  skill_level: 'beginner' | 'improver' | 'intermediate' | 'advanced';
  ranking?: string | null;
  bio: string;
  gender_preference: 'women' | 'men' | 'all';
  min_age: number;
  max_age: number;
  skill_tolerance: 0 | 1;
  availability: Availability[];
  instagram_handle?: string | null;
  age?: number;
};

export type Photo = {
  photo_id: string;
  display_order: number;
  status: 'pending' | 'approved' | 'rejected';
};

export type DiscoveryCard = {
  player: {
    player_id: string;
    display_name: string;
    age: number;
    gender: 'woman' | 'man';
    skill_level: string;
    bio: string;
    play_summary: string;
    photo_urls: string[];
    distance_miles: number;
  };
};

export type Match = {
  match_id: string;
  player_id: string;
  display_name: string;
  matched_at: string;
  has_instagram: boolean;
  photo_urls: string[];
};

export type Report = {
  report_id: string;
  target_id: string;
  display_name: string;
  category: string;
  details: string;
  status: string;
  created_at: string;
  review_due_at: string;
  overdue: boolean;
  reviewed_at?: string;
};

export const reportCategories = [
  ['fake_profile', 'Fake profile or photos'],
  ['inappropriate_photos', 'Inappropriate photos'],
  ['harassment', 'Harassment or abuse'],
  ['contact_details', 'Contact details in the profile'],
  ['underage', 'Looks like someone under 18'],
  ['other', 'Something else']
] as const;

export function distanceLabel(miles: number): string {
  return miles === 0 ? 'Under 1 mile' : `${miles} ${miles === 1 ? 'mile' : 'miles'}`;
}

export function weekdayLabel(day: number): string {
  return ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'][day - 1] ?? '';
}

export function formatDate(value: string): string {
  return new Intl.DateTimeFormat('en-GB', {
    dateStyle: 'medium',
    timeZone: 'Europe/London'
  }).format(new Date(value));
}
