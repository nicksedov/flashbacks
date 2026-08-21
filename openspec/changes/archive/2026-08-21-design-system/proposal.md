## Why

The webapp UI was built incrementally without a single source of truth for visual language and layout. The same concept is implemented several different ways across screens (two `Select` components, two `Tabs` styles, ad-hoc raw `<button>` elements alongside the `Button` primitive, three different empty-state layouts, mixed `alert()`/`toast` feedback, and inconsistent placement of "add item" actions). This makes the product feel inconsistent, slows down new feature work (agents have no reference for how to render a list, a settings form, or a toolbar), and risks introducing more drift. This change establishes a canonical, Material-3-inspired design system as a project capability so agents and developers can reference one authoritative spec when building features.

## What Changes

- Add a new `design-system` capability whose specification defines the project's visual and interaction language, grounded in Google Material Design 3 and implemented with React + Tailwind CSS 4 + Radix UI.
- Define the design tokens: color roles (background, surface/card, primary, secondary, muted, accent, border, input, ring, destructive), typography scale, spacing scale, corner radius, elevation/shadow, and focus states — mapped onto the existing CSS variables in [`globals.css`](../../webapp/src/styles/globals.css).
- Define the canonical component primitives and their variants/states (button, icon button, badge, input, select, checkbox, radio, switch, tabs, card, dialog, tooltip, skeleton, progress, empty state, pagination footer, toast) and when to use each variant.
- Define layout and positioning rules: page chrome (sidebar/header/main), view header + actions, toolbar placement, primary action ("add") placement, list/card item actions, dialog footer conventions, and grid/responsive breakpoints.
- Define unified interaction patterns: destructive actions require confirmation, settings manipulation follows a single save pattern, feedback uses the `sonner` toast system (not `alert()`/`confirm()`), and loading/empty/error states are standardized.
- Define theming requirements: dark/light variants per accent hue must remain token-driven, components must never hardcode colors or bypass tokens (e.g., the dialog's `bg-white/90` vs `bg-black/90` shortcut).
- Define accessibility and "constraints to avoid typical UI design errors" (no magic sizes, no raw strings outside i18n, no text-only color signaling, minimum touch targets, etc.).
- Record the inventory of concrete inconsistencies found in the current code and the target resolution for each (the reconciliation backlog), so they can be fixed in follow-up changes.

## Capabilities

### New Capabilities

- `design-system`: The project's design system specification — design tokens, component primitive contracts and variants, layout/positioning principles, interaction patterns, responsive behavior, theming rules, accessibility requirements, and UI anti-pattern constraints. Agents building new webapp features reference this spec for consistent implementation.

### Modified Capabilities

<!-- none -->

## Impact

- Frontend webapp: [`src/styles/globals.css`](../../webapp/src/styles/globals.css), [`src/components/ui/*`](../../webapp/src/components/ui/), [`src/components/gallery/*`](../../webapp/src/components/gallery/), [`src/components/settings/*`](../../webapp/src/components/settings/), [`src/components/layout/*`](../../webapp/src/components/layout/), [`src/components/tabs/*`](../../webapp/src/components/tabs/), [`src/theme/*`](../../webapp/src/theme/).
- Documentation: the new master spec at `openspec/specs/design-system/spec.md` (created when the change is applied), referenced from the root [`AGENTS.md`](../../AGENTS.md) guidance.
- No backend, API contract, OpenAPI, or MCP changes. No new third-party dependencies — stays on Radix UI, Tailwind CSS 4, class-variance-authority, sonner, and lucide-react.
- i18n: only if the reconciliation backlog fixes hardcoded English strings (e.g., `ProviderConfigForm` labels); en/ru updated together when that happens.

## Non-goals

- Not a full visual redesign: the existing shadcn/ui-based primitives and the 9-theme token set are kept; this change standardizes and documents them rather than replacing them.
- Not a component-library migration: no new UI framework (e.g., MUI, Chakra) is introduced.
- Not a one-shot refactor: fixing every existing inconsistency is split into small, separately reviewable follow-up changes tracked in the reconciliation backlog; this change only defines the standard and the backlog.
- Not changing product behavior, data models, or API contracts.
