import { GraphNode } from '../../models/project.model';

export const LANG_COLORS: Record<string, string> = {
  go: '#00ADD8',
  python: '#3572A5',
  typescript: '#3178C6',
  javascript: '#F7DF1E',
  java: '#B07219',
  rust: '#DEA584',
  cpp: '#F34B7D',
  c: '#555555',
  csharp: '#178600',
  ruby: '#CC342D',
  php: '#4F5D95',
  swift: '#FA7343',
  kotlin: '#7F52FF',
  vue: '#41B883',
  html: '#E34C26',
  css: '#563D7C',
  scss: '#C6538C',
  yaml: '#CB171E',
  json: '#A0A0A0',
  markdown: '#083FA1',
  shell: '#89E051',
  dockerfile: '#384D54',
  text: '#6E7681',
};

export function langColor(lang: string, type: string): string {
  if (type === 'directory') return '#58A6FF';
  return LANG_COLORS[lang] ?? LANG_COLORS['text'];
}

export function nodeRadius(d: GraphNode): number {
  if (d.type === 'directory') return 22 + Math.min((d.children || 0) * 1.2, 18);
  return 9 + Math.min((d.lines || 0) / 300, 12);
}
