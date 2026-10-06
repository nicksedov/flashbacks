# Branding Specification

## Purpose

Defines where the official Flashbacks branded logos appear in the webapp — the welcome-screen logo on the auth screen and the base logo at the top of the sidebar — together with their rendering, scaling, and collapsed-state behavior so the product identity is consistent and unobtrusive.

## Requirements

### Requirement: Auth screen shows the welcome-screen logo

The webapp auth screen (login, registration, and bootstrap modes) SHALL render the branded welcome-screen logo from `flashbacks_logo_welcomescreen.png` at the top of the auth card, replacing the previous icon-and-title block. The logo SHALL be scaled proportionally so that its rendered width fits within the card's content width (upscaled to `w-72` = 288px on a `max-w-md` card, 50% larger than the original `w-48`) while preserving aspect ratio; the surrounding descriptive text remains below the logo.

#### Scenario: Login screen shows the branded logo

- **WHEN** the user opens the login screen (not authenticated)
- **THEN** the welcome-screen logo is displayed centered at the top of the auth card instead of the previous icon and "Flashbacks" title

#### Scenario: Registration and bootstrap modes show the logo too

- **WHEN** the user switches to the registration tab or the app is in bootstrap setup mode
- **THEN** the same welcome-screen logo is shown, with only the mode-specific descriptive text changing

#### Scenario: Logo stays proportional

- **WHEN** the logo is rendered on a card whose width is between 320px and 480px
- **THEN** the image keeps its original aspect ratio (height auto) and is not distorted or clipped

### Requirement: Sidebar header shows the base logo

The sidebar header (top area, above the navigation) SHALL render the branded base logo from `flashbacks_logo_base.png` instead of the inline icon glyph and "Flashbacks" text. In the expanded sidebar the logo SHALL be scaled to fit within the header's height and horizontal padding while preserving aspect ratio (upscaled to `h-16` = 64px, matching the header height — 100% larger than the original `h-8`), and SHALL not force the header to grow taller than its current height.

#### Scenario: Expanded sidebar shows the base logo

- **WHEN** the sidebar is expanded
- **THEN** the base logo is displayed at the top of the sidebar, replacing the previous icon-and-title block, and the header height is not increased

#### Scenario: Collapsed sidebar keeps a compact mark

- **WHEN** the sidebar is collapsed to its narrow rail width
- **THEN** the full wordmark logo is not shown because it cannot fit; instead a small centered logo mark (the existing icon glyph) is displayed, so the collapsed header remains visually compact

### Requirement: Logo images are accessible

The logo images SHALL be rendered as `<img>` elements with a meaningful `alt`/`aria-label` describing the product, and SHALL not carry interactive behavior or duplicated brand text adjacent to them.

#### Scenario: Screen reader reads the logo

- **WHEN** a screen reader encounters the logo image
- **THEN** it announces a descriptive label (for example, "Flashbacks") and no unlabeled image is left on the auth screen or sidebar

### Requirement: Browser favicon uses the branded icon

The webapp SHALL serve the browser favicon from the official branded icon source so the browser tab displays the brand mark. The favicon SHALL be derived from the branded source image and referenced from the HTML head so it loads without user interaction.

#### Scenario: Browser tab shows the branded favicon

- **WHEN** the user opens the webapp in a browser
- **THEN** the browser tab displays the branded icon as the favicon, replacing the previous generic glyph

#### Scenario: Favicon asset is cached by the service worker

- **WHEN** the webapp service worker precaches static assets
- **THEN** the favicon asset is included in the precache list

### Requirement: PWA app icons use the branded icon

The webapp SHALL generate and serve all PWA app icons (all declared sizes, the Apple touch icon, and maskable variants) from the official branded icon source. Regular icons SHALL center the brand mark within the canvas, and maskable icons SHALL fit the brand mark within the safe zone so it is not clipped by platform mask shapes.

#### Scenario: Installed PWA shows the branded icon

- **WHEN** a user installs the webapp as a PWA or adds it to the home screen
- **THEN** the launcher/home-screen icon is the branded icon instead of the previous generic glyph

#### Scenario: Maskable icons keep the mark in the safe zone

- **WHEN** a platform renders a maskable PWA icon
- **THEN** the branded mark stays within the safe zone and is not clipped by the platform mask

#### Scenario: Manifest references the generated icons

- **WHEN** the browser reads the web app manifest
- **THEN** the manifest lists the regenerated icon files with matching sizes and MIME types
