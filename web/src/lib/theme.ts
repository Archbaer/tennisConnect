import { get, writable } from 'svelte/store';

export type Theme = 'light' | 'dark';

const storageKey = 'tennisconnect_theme';

export const theme = writable<Theme>('light');

let preference: Theme | null = null;

function systemTheme(): Theme {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function storedTheme(): Theme | null {
  try {
    const value = localStorage.getItem(storageKey);
    return value === 'light' || value === 'dark' ? value : null;
  } catch {
    return null;
  }
}

function apply(value: Theme): void {
  document.documentElement.dataset.bsTheme = value;
  theme.set(value);
}

export function initTheme(): () => void {
  if (typeof window === 'undefined') return () => {};
  preference = storedTheme();
  apply(preference ?? systemTheme());
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const onChange = () => {
    if (!preference) apply(systemTheme());
  };
  media.addEventListener('change', onChange);
  return () => media.removeEventListener('change', onChange);
}

export function toggleTheme(): void {
  if (typeof window === 'undefined') return;
  const next: Theme = get(theme) === 'dark' ? 'light' : 'dark';
  preference = next;
  try {
    localStorage.setItem(storageKey, next);
  } catch {
    // A blocked storage API only loses the preference.
  }
  apply(next);
}
