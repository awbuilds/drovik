import type { ReactNode } from 'react'
import { Badge } from '@/components/ui/Badge'
import { Logo } from '@/components/ui/Logo'

interface AuthLayoutProps {
  eyebrow: string
  title: string
  description: string
  children: ReactNode
  footer?: ReactNode
}

const terminalLines = [
  ['argon2id', 'active'],
  ['sessions', 'server-side'],
  ['csrf', 'enforced'],
  ['telemetry', 'off'],
]

export function AuthLayout({ eyebrow, title, description, children, footer }: AuthLayoutProps) {
  return (
    <div className="bg-ink-950 min-h-screen lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,1.05fr)]">
      <main className="relative flex min-h-screen flex-col px-6 py-8 sm:px-10 lg:px-14">
        <div className="flex items-center justify-between">
          <Logo />
          <Badge variant="outline">Phase 1 preview</Badge>
        </div>

        <div className="mx-auto flex w-full max-w-sm flex-1 flex-col justify-center py-12">
          <p className="text-signal-400 font-mono text-xs tracking-[0.2em] uppercase">{eyebrow}</p>
          <h1 className="font-display text-ink-50 mt-3 text-3xl font-semibold tracking-tight text-balance">
            {title}
          </h1>
          <p className="text-ink-400 mt-3 text-sm leading-6">{description}</p>
          <div className="mt-8">{children}</div>
          {footer ? <div className="text-ink-400 mt-8 text-xs">{footer}</div> : null}
        </div>

        <p className="text-ink-500 font-mono text-[10px] tracking-wider uppercase">
          Self-hosted · Agentless · AGPL-3.0
        </p>
      </main>

      <aside className="bg-ink-900 border-ink-800 relative hidden overflow-hidden border-l lg:block">
        <div className="grid-backdrop absolute inset-0 opacity-70" aria-hidden="true" />
        <div
          className="bg-signal-500/10 absolute -top-40 right-[-10%] h-[520px] w-[520px] rounded-full blur-[120px]"
          aria-hidden="true"
        />
        <div className="relative flex h-full flex-col justify-between p-14">
          <div className="flex items-center gap-3">
            <span className="bg-signal-400 h-2 w-2 animate-pulse rounded-full" />
            <span className="text-ink-400 font-mono text-[11px] tracking-[0.24em] uppercase">
              Control surface / online
            </span>
          </div>

          <div className="max-w-lg">
            <p className="text-ink-500 font-mono text-xs tracking-[0.2em] uppercase">
              A calmer way to operate
            </p>
            <h2 className="font-display text-ink-50 mt-4 text-5xl leading-[1.05] font-semibold tracking-[-0.03em] text-balance">
              Your servers.
              <br />
              <span className="text-signal-400">One deliberate surface.</span>
            </h2>
            <p className="text-ink-300 mt-5 max-w-md text-sm leading-6">
              Drovik uses the SSH access you already have. No agent on your machines. No hidden
              privilege. No fabricated success.
            </p>

            <div className="border-ink-700 bg-ink-950/70 mt-10 rounded-lg border p-4 shadow-2xl backdrop-blur">
              <div className="border-ink-700 flex items-center gap-2 border-b pb-3">
                <span className="h-2.5 w-2.5 rounded-full bg-rose-400" />
                <span className="bg-signal-400 h-2.5 w-2.5 rounded-full" />
                <span className="bg-mint-400 h-2.5 w-2.5 rounded-full" />
                <span className="text-ink-500 ml-2 font-mono text-[10px]">
                  drovik@control — security posture
                </span>
              </div>
              <dl className="mt-3 space-y-2 font-mono text-xs">
                {terminalLines.map(([key, value]) => (
                  <div key={key} className="flex items-center justify-between gap-4">
                    <dt className="text-ink-500">{key}</dt>
                    <dd className="text-mint-300">{value}</dd>
                  </div>
                ))}
              </dl>
            </div>
          </div>

          <div className="text-ink-500 flex items-center justify-between font-mono text-[10px] tracking-wider uppercase">
            <span>Built for Linux operators</span>
            <span>No telemetry by default</span>
          </div>
        </div>
      </aside>
    </div>
  )
}
