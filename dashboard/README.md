# sblite Dashboard

Modern React dashboard for sblite, built with Vite, React, TypeScript, Tailwind CSS, and shadcn/ui.

## Tech Stack

- **Vite** - Fast build tool and dev server
- **React** - UI library
- **TypeScript** - Type safety
- **Tailwind CSS v4** - Utility-first CSS
- **shadcn/ui** - High-quality React components
- **React Router** - Client-side routing
- **Lucide React** - Icon library

## Development

```bash
# Install dependencies
npm install

# Start dev server (proxies API to Go backend on port 8080)
npm run dev

# Type checking
tsc --noEmit
```

The dev server runs on `http://localhost:5173` and proxies API requests to the Go backend on `http://localhost:8080`.

## Building for Production

```bash
# Build dashboard to dist/
npm run build
```

The built files are then copied to `internal/dashboard/assets/dist` and embedded in the Go binary.

## Go Integration

The dashboard is embedded in the Go binary using `//go:embed` in `internal/dashboard/assets/assets.go`.

To build the complete sblite binary with the new dashboard:

```bash
# From the sblite root directory
cd dashboard && npm run build && cd ..
cp -r dashboard/dist internal/dashboard/assets/
go build -o sblite .
```

Or use the Makefile target:

```bash
make build-dashboard
```

## File Structure

```
dashboard/
├── src/
│   ├── components/
│   │   ├── layout/     # Layout components (sidebar, header)
│   │   └── ui/         # shadcn/ui components
│   ├── lib/            # Utilities (API client, cn function)
│   ├── pages/          # Page components
│   ├── App.tsx         # Main app with routing
│   └── main.tsx        # Entry point
├── public/             # Static assets
├── index.html          # HTML shell
├── vite.config.ts      # Vite configuration (with API proxy)
├── tailwind.config.js  # Tailwind configuration
└── package.json        # Dependencies

internal/dashboard/assets/
├── static/             # Legacy vanilla JS dashboard (fallback)
└── dist/               # React dashboard build output
```

## Adding shadcn Components

```bash
npx shadcn@latest add [component-name]
```

Components are added to `src/components/ui/` and use the local `lib/utils.ts` for styling utilities.

## API Client

The `src/lib/api.ts` file provides a typed API client for all dashboard endpoints:

```typescript
import { api } from '@/lib/api'

const tables = await api.getTables()
const users = await api.getUsers({ limit: 10 })
```

## Theming

The dashboard uses CSS custom properties for theming, defined in `src/index.css`. Light and dark modes are supported via `@media (prefers-color-scheme: dark)`.
