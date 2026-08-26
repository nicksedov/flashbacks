## ADDED Requirements

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
