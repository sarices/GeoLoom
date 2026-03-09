import { ShellCard, SectionTitle } from './shell-card'

export function JsonPanel({ title, data, description }: { title: string; data: unknown; description?: string }) {
  return (
    <ShellCard tone="muted">
      <SectionTitle title={title} description={description} />
      <pre className="overflow-x-auto rounded-[24px] bg-shell-strong p-4 text-xs leading-6 text-text-soft">{JSON.stringify(data, null, 2)}</pre>
    </ShellCard>
  )
}
