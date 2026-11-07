# Drovik Web

The Drovik browser client: React 19, TypeScript (strict), Vite, Tailwind CSS 4 and Radix
primitives.

## Develop

```bash
pnpm install --frozen-lockfile
pnpm dev
```

The dev server listens on <http://localhost:5173> and proxies `/api` to
`http://127.0.0.1:8080`. Override the backend target with `DROVIK_API_PROXY`.

## Verify

```bash
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```

## Design

The interface follows the “control room” direction in `../AGENTS.md`: deep ink surfaces, a warm
signal accent, self-hosted Inter/Space Grotesk/JetBrains Mono, restrained motion, and accessible
Radix behavior. All colors, spacing, radii and motion come from the semantic tokens in
`src/styles/tokens.css`; components must not hardcode raw values.
