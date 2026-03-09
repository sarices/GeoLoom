import type { ReactNode } from 'react'

export function Badge({ children, tone = 'neutral' }: { children: ReactNode; tone?: 'neutral' | 'good' | 'warn' | 'bad' }) {
  const toneClass = {
    neutral: 'bg-shell text-text-soft',
    good: 'bg-accent-main/12 text-accent-main',
    warn: 'bg-accent-warm/18 text-[#8a6700] dark:text-accent-warm',
    bad: 'bg-accent-coral/15 text-[#a14825] dark:text-accent-coral',
  }[tone]

  return <span className={`inline-flex rounded-full px-3 py-1 text-xs font-medium ${toneClass}`}>{children}</span>
}
