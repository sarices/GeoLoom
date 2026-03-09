import { ShellCard } from './shell-card'

export function MetricCard({
  label,
  value,
  accent,
  hint,
}: {
  label: string
  value: string | number
  accent: 'mint' | 'gold' | 'coral'
  hint?: string
}) {
  const accentMap = {
    mint: 'bg-accent-main/12 text-accent-main',
    gold: 'bg-accent-warm/20 text-[#8a6700] dark:text-accent-warm',
    coral: 'bg-accent-coral/18 text-[#a14825] dark:text-accent-coral',
  }

  return (
    <ShellCard className="min-h-[172px]">
      <div className="flex h-full flex-col justify-between gap-7">
        <div className={`inline-flex w-fit rounded-full px-3 py-1 text-[11px] font-semibold tracking-[0.08em] uppercase ${accentMap[accent]}`}>{label}</div>
        <div>
          <div className="font-display text-[2.6rem] leading-none tracking-[-0.05em] text-text-main">{value}</div>
          {hint ? <div className="mt-2.5 text-sm leading-6 text-text-soft">{hint}</div> : null}
        </div>
      </div>
    </ShellCard>
  )
}
