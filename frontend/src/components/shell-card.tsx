import type { PropsWithChildren, ReactNode } from 'react'

export function ShellCard({ children, tone = 'default', className = '' }: PropsWithChildren<{ tone?: 'default' | 'muted' | 'accent'; className?: string }>) {
  const toneClass =
    tone === 'muted'
      ? 'bg-card-muted'
      : tone === 'accent'
        ? 'bg-[color:color-mix(in_srgb,var(--accent)_9%,white_91%)] dark:bg-[color:color-mix(in_srgb,var(--accent)_16%,#12201f_84%)]'
        : 'bg-card'

  return <section className={`rounded-[30px] border border-line-soft/60 ${toneClass} p-6 shadow-card-soft ${className}`}>{children}</section>
}

export function SectionTitle({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="mb-5 flex items-start justify-between gap-3">
      <div>
        <h2 className="font-display text-[1.15rem] font-semibold tracking-[0.01em] text-text-main">{title}</h2>
        {description ? <p className="mt-1.5 text-sm leading-6 text-text-soft">{description}</p> : null}
      </div>
      {action}
    </div>
  )
}
