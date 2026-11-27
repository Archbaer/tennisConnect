import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const candidates = [resolve(process.cwd(), 'src/app.css'), resolve(process.cwd(), 'web/src/app.css')];
const cssPath = candidates.find((path) => existsSync(path));
if (!cssPath) throw new Error(`Could not find app.css in ${process.cwd()}`);
const css = readFileSync(cssPath, 'utf8');

type Theme = 'light' | 'dark';

function block(source: string, selector: string): string {
  const start = source.indexOf(selector);
  if (start < 0) throw new Error(`Missing selector ${selector}`);
  const open = source.indexOf('{', start);
  let depth = 0;
  for (let index = open; index < source.length; index += 1) {
    if (source[index] === '{') depth += 1;
    if (source[index] === '}') {
      depth -= 1;
      if (depth === 0) return source.slice(open + 1, index);
    }
  }
  throw new Error(`Unclosed selector ${selector}`);
}

function tokens(scope: string): Map<string, string> {
  const found = new Map<string, string>();
  for (const match of scope.matchAll(/--([a-z0-9-]+):\s*([^;]+);/gi)) {
    found.set(match[1], match[2].trim());
  }
  return found;
}

const themes: Record<Theme, Map<string, string>> = {
  light: tokens(block(css, ':root {')),
  dark: tokens(block(css, "html[data-bs-theme='dark'] {"))
};

function rgb(hex: string): [number, number, number] {
  const value = /^#[0-9a-f]{6}$/i.exec(hex.trim());
  if (!value) throw new Error(`Not a six-digit hex colour: ${hex}`);
  return [1, 3, 5].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16)) as [
    number,
    number,
    number
  ];
}

function luminance(hex: string): number {
  const [red, green, blue] = rgb(hex).map((channel) => {
    const value = channel / 255;
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
}

function contrast(foreground: string, background: string): number {
  const values = [luminance(foreground), luminance(background)].sort((left, right) => right - left);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

function token(theme: Theme, name: string): string {
  const value = themes[theme].get(name);
  if (!value) throw new Error(`Missing --${name} in ${theme} theme`);
  return value;
}

const textPairs = [
  ['body text', 'ink', 'paper'],
  ['text on surface', 'ink', 'surface'],
  ['muted text', 'muted', 'paper'],
  ['muted text on surface', 'muted', 'surface'],
  ['link on background', 'link', 'paper'],
  ['CTA text on optic yellow', 'ball-ink', 'ball'],
  ['CTA text on yellow hover', 'ball-ink', 'ball-deep'],
  ['badge text', 'chip-ink', 'chip-bg'],
  ['success badge', 'success', 'success-bg'],
  ['danger badge', 'danger', 'danger-bg'],
  ['info badge', 'info-ink', 'info-surface'],
  ['warning badge', 'clay-ink', 'clay-bg'],
  ['danger button label', 'on-danger', 'danger-fill'],
  ['footer text', 'on-court', 'court-deep'],
  ['avatar initials on court', 'ball', 'court-deep'],
  ['avatar initials on clay', 'on-danger', 'clay-deep']
] as const;

const interfacePairs = [
  ['focus ring', 'sky', 'paper'],
  ['focus ring on surface', 'sky', 'surface'],
  ['control border', 'control-border', 'paper'],
  ['control border on surface', 'control-border', 'surface']
] as const;

describe.each<Theme>(['light', 'dark'])('%s theme contrast', (theme) => {
  it('keeps body copy and badges at WCAG AA', () => {
    for (const [label, foreground, background] of textPairs) {
      const ratio = contrast(token(theme, foreground), token(theme, background));
      expect(ratio, `${label}: --${foreground} on --${background}`).toBeGreaterThanOrEqual(4.5);
    }
  });

  it('keeps focus and control boundaries at 3:1', () => {
    for (const [label, foreground, background] of interfacePairs) {
      const ratio = contrast(token(theme, foreground), token(theme, background));
      expect(ratio, `${label}: --${foreground} on --${background}`).toBeGreaterThanOrEqual(3);
    }
  });
});
