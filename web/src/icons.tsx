const base = (n: number) => ({
  width: n,
  height: n,
  viewBox: '0 0 24 24',
  fill: 'none' as const,
  stroke: 'currentColor',
  strokeWidth: 2,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
})

export const Triangle = () => (
  <svg width={11} height={11} viewBox="0 0 12 12" aria-hidden="true">
    <path d="M3.5 1.5 9 6l-5.5 4.5z" fill="currentColor" stroke="none" />
  </svg>
)
export const SearchIcon = () => (
  <svg {...base(15)} aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></svg>
)
export const XIcon = () => (
  <svg {...base(13)} aria-hidden="true"><path d="M18 6 6 18M6 6l12 12" /></svg>
)
export const ArrowUpRight = () => (
  <svg {...base(12)} aria-hidden="true"><path d="M7 17 17 7M8 7h9v9" /></svg>
)
export const Check = () => (
  <svg {...base(12)} aria-hidden="true"><path d="M20 6 9 17l-5-5" /></svg>
)
export const Sun = () => (
  <svg {...base(15)} aria-hidden="true"><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></svg>
)
export const Moon = () => (
  <svg {...base(15)} aria-hidden="true"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" /></svg>
)
export const Menu = () => (
  <svg {...base(17)} aria-hidden="true"><path d="M4 6h16M4 12h16M4 18h16" /></svg>
)
