# Flashbacks WebUI

Frontend application for the Flashbacks image management platform.

## Stack

- **React 19** - UI framework
- **TypeScript 6** - Type safety
- **Vite 8** - Build tool and dev server
- **Tailwind CSS 4** - Styling
- **Radix UI** - Accessible component primitives
- **React Leaflet** - Map visualization
- **Vitest** - Testing framework

## Quick Start

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Run tests
npm test
```

## Development

The development server runs on `http://localhost:5180` and proxies API requests to `http://localhost:5170`.

### Project Structure

```
webui/
├── public/              # Static assets
├── src/
│   ├── components/      # Reusable UI components
│   ├── hooks/           # Custom React hooks
│   ├── i18n/            # Internationalization
│   ├── lib/             # Utility functions
│   ├── providers/       # Context providers
│   ├── theme/           # Theme configuration
│   ├── types/           # TypeScript type definitions
│   ├── utils/           # Helper functions
│   ├── App.tsx          # Root component
│   └── main.tsx         # Application entry point
├── package.json
├── tsconfig.json
└── vite.config.ts
```

## Build

```bash
# Production build
npm run build

# Preview production build
npm run preview
```

## Testing

```bash
# Run tests once
npm test

# Run tests in watch mode
npm run test:watch
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_URL` | API service URL | `/api` (proxied to localhost:5170) |

## API Integration

WebUI communicates with the API service via REST endpoints defined in [`docs/api-contracts/api-service.yaml`](../docs/api-contracts/api-service.yaml).

The Vite dev server proxies `/api` requests to the backend service automatically.

## Linting

```bash
# Run ESLint
npm run lint
```

## License

MIT
