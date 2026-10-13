import { createSignal } from 'solid-js';

type Theme = 'dark' | 'light';

const stored = typeof localStorage !== 'undefined' ? (localStorage.getItem('theme') as Theme | null) : null;
const initial: Theme = stored || 'dark';
if (typeof document !== 'undefined') {
  document.documentElement.setAttribute('data-theme', initial);
}

const [theme, setThemeSignal] = createSignal<Theme>(initial);

export function toggleTheme() {
  const next = theme() === 'dark' ? 'light' : 'dark';
  setThemeSignal(next);
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', next);
  }
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem('theme', next);
  }
}

export { theme };
