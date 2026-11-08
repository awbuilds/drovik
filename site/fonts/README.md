# Website fonts

These files are the Latin variable subsets used by the Drovik application's
design system:

| File | Family | Source package | License |
|---|---|---|---|
| `inter-latin-wght-normal.woff2` | Inter Variable | `@fontsource-variable/inter` 5.3.x | SIL OFL 1.1 (`LICENSE-Inter.txt`) |
| `space-grotesk-latin-wght-normal.woff2` | Space Grotesk Variable | `@fontsource-variable/space-grotesk` 5.3.x | SIL OFL 1.1 (`LICENSE-SpaceGrotesk.txt`) |
| `jetbrains-mono-latin-wght-normal.woff2` | JetBrains Mono Variable | `@fontsource-variable/jetbrains-mono` 5.3.x | SIL OFL 1.1 (`LICENSE-JetBrainsMono.txt`) |

The files were copied from the versions already used by `frontend/` so the site
and the application render the same typography without a third-party font
request. The license files must stay beside the fonts when the site is
redistributed.

Only the Latin subset is included; the site is English-only. The exact pinned
versions are in `frontend/pnpm-lock.yaml`. When updating the application's font
packages, copy the matching `*-latin-wght-normal.woff2` files here and update
the `@font-face` declarations in `../css/style.css` in the same change.
