const radiusKey = 'tennisconnect_radius';
const allowed = [5, 10, 20, 30];

export function loadRadius(fallback = 10): number {
  try {
    const value = Number(localStorage.getItem(radiusKey));
    return allowed.includes(value) ? value : fallback;
  } catch {
    return fallback;
  }
}

export function saveRadius(value: number): void {
  try {
    localStorage.setItem(radiusKey, String(value));
  } catch {
    // A blocked storage API only loses the preference.
  }
}
