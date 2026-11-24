import { writable } from 'svelte/store';

export const online = writable(typeof navigator === 'undefined' ? true : navigator.onLine);

export function watchConnectivity(): () => void {
  if (typeof window === 'undefined') return () => {};
  const wentOnline = () => online.set(true);
  const wentOffline = () => online.set(false);
  window.addEventListener('online', wentOnline);
  window.addEventListener('offline', wentOffline);
  return () => {
    window.removeEventListener('online', wentOnline);
    window.removeEventListener('offline', wentOffline);
  };
}
