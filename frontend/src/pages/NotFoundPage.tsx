import { Link } from 'react-router-dom'
import { Compass } from 'lucide-react'
import { Button } from '@/components/ui/Button'

export function NotFoundPage() {
  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <div className="surface max-w-md p-8 text-center">
        <div className="bg-ink-800 text-signal-300 mx-auto flex h-12 w-12 items-center justify-center rounded-full">
          <Compass className="h-6 w-6" aria-hidden="true" />
        </div>
        <p className="text-ink-500 mt-4 font-mono text-[10px] tracking-[0.2em] uppercase">
          404 / off the map
        </p>
        <h1 className="font-display text-ink-50 mt-2 text-2xl font-semibold">
          This panel does not exist
        </h1>
        <p className="text-ink-400 mt-2 text-sm">
          The route you opened is not part of this build. Return to the command center.
        </p>
        <Button asChild className="mt-6">
          <Link to="/">Back to command center</Link>
        </Button>
      </div>
    </div>
  )
}
