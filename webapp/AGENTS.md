# WebUI Agent Context

## Overview

This is the frontend application for the Flashbacks platform, a React 19 + TypeScript 6 SPA built with Vite 8 and Tailwind CSS 4.

## Stack

- React 19 with hooks and context API
- TypeScript 6 (strict mode)
- Vite 8 for builds and dev server
- Tailwind CSS 4 for styling
- Radix UI for accessible primitives
- React Leaflet for map components
- Vitest for testing

## Structure

```
webui/
├── src/
│   ├── components/      # Reusable UI components
│   ├── hooks/           # Custom React hooks
│   ├── i18n/            # Internationalization (en, ru)
│   ├── lib/             # Utility functions and constants
│   ├── providers/       # Context providers (Auth, Settings)
│   ├── theme/           # Theme context and provider
│   ├── types/           # Shared TypeScript types
│   ├── utils/           # Helper functions
│   ├── App.tsx          # Root component with routing
│   └── main.tsx         # Application entry point
├── public/              # Static assets and PWA files
├── package.json
├── tsconfig.json
├── vite.config.ts
└── vitest.config.ts
```

## Architecture

### State Management

- **Context API**: Used for global state (Auth, Settings, Theme)
- **Local state**: No global state library; prefer lifting state or context
- **Data fetching**: Custom hooks with fetch API

### Component Patterns

- **Composition**: Prefer composition over inheritance
- **Props**: Use interfaces, keep them focused
- **Hooks**: Extract reusable logic into custom hooks

### Internationalization (i18n)

- Located in `src/i18n/`
- Supports English (`en`) and Russian (`ru`)
- Uses custom translation system with type safety
- Translation keys must be added to both `translations.en.ts` and `translations.ru.ts`

### Styling

- Tailwind CSS 4 with utility-first approach
- Radix UI primitives for accessible components
- Theme variables defined in `src/theme/`
- Dark mode support via CSS variables

## TypeScript Guidelines

### DO

- Use strict mode
- Define interfaces for component props
- Use type guards for runtime validation
- Export types from `src/types/index.ts`
- Use `as const` for literal types

### DO NOT

- Use `any` type
- Use type assertions (`as Type`) without validation
- Ignore TypeScript errors with `@ts-ignore`
- Use `enum`; prefer union types with `as const`

## Testing

- Write tests for utility functions and hooks
- Use Vitest for unit tests
- Test user interactions, not implementation details
- Mock external dependencies

## Commands

```bash
npm install          # Install dependencies
npm run dev          # Start dev server (port 5180)
npm run build        # Production build
npm run lint         # Run ESLint
npm test             # Run tests
npm run test:watch   # Tests in watch mode
npm run preview      # Preview production build
```

## Environment

- Node.js 20+
- npm 10+

## API Integration

- API base URL: `/api` (proxied via Vite)
- OpenAPI spec: `docs/api-contracts/api-service.yaml`
- Use fetch API with proper error handling
- Include CSRF token in headers for mutations

## Key Features

- **Authentication**: Session-based with CSRF protection
- **Gallery**: Image grid with filtering and pagination
- **Map View**: GPS coordinates visualization with Leaflet
- **OCR**: Text recognition interface
- **AI Chat**: Conversational search agent
- **Settings**: User preferences management
- **PWA**: Progressive web app support

## Validation

- Use type-safe form validation
- Validate on both client and server
- Display user-friendly error messages
- Use `sonner` for toast notifications