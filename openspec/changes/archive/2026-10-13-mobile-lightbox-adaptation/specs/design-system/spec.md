# Spec Delta

## ADDED Requirements

### Requirement: Mobile lightbox is image-first

On a viewport below the `md` breakpoint a lightbox SHALL render the image as the primary content that fills the lightbox area, and any lightbox detail panel SHALL NOT occupy a permanent side column beside the image.

<!-- focused-spec:id=design-system.mobile-lightbox-image-first -->
#### Scenario: Image fills the lightbox on mobile

- **WHEN** a lightbox is opened on a viewport below the `md` breakpoint
- **THEN** the image is displayed within the lightbox area and no detail panel occupies a side column beside it

### Requirement: Mobile lightbox panel is collapsible

On a viewport below the `md` breakpoint a lightbox that has a detail panel SHALL show it collapsed by default and SHALL let the user open and close it through a single toggle control inside the lightbox, and the toggle SHALL keep a consistent position that stays reachable in both states.

<!-- focused-spec:id=design-system.mobile-lightbox-panel-collapsed -->
#### Scenario: Panel is collapsed when the lightbox opens

- **WHEN** a lightbox with a detail panel is opened on a viewport below the `md` breakpoint
- **THEN** the detail panel is collapsed and only the image and the toggle control are shown

<!-- focused-spec:id=design-system.mobile-lightbox-panel-toggle -->
#### Scenario: Toggle expands the collapsed panel

- **WHEN** the user activates the panel toggle while the panel is collapsed on a mobile viewport
- **THEN** the detail panel is expanded

<!-- focused-spec:id=design-system.mobile-lightbox-panel-collapse -->
#### Scenario: Toggle collapses the expanded panel

- **WHEN** the user activates the panel toggle while the panel is expanded on a mobile viewport
- **THEN** the detail panel is collapsed and the image is fully visible again

<!-- focused-spec:id=design-system.mobile-lightbox-toggle-placement -->
#### Scenario: Toggle stays reachable in both panel states

- **WHEN** the lightbox panel is collapsed or expanded on a mobile viewport
- **THEN** the toggle control is rendered at a consistent position that remains reachable in both states

### Requirement: Expanding the panel does not resize the image

On a viewport below the `md` breakpoint, toggling the lightbox detail panel SHALL NOT change the rendered size or position of the image.

<!-- focused-spec:id=design-system.mobile-lightbox-image-unchanged -->
#### Scenario: Image keeps its size when the panel expands

- **WHEN** the detail panel is expanded on a mobile viewport
- **THEN** the image keeps the same rendered size and position as when the panel was collapsed

### Requirement: Expanded mobile lightbox panel overlays the image

When expanded on a viewport below the `md` breakpoint, the lightbox detail panel SHALL cover approximately 80% of the lightbox area and SHALL be drawn over the image, with a scrim separating the panel from the image area that remains visible beside it.

<!-- focused-spec:id=design-system.mobile-lightbox-panel-overlay -->
#### Scenario: Expanded panel covers about 80% over the image

- **WHEN** the detail panel is expanded on a mobile viewport
- **THEN** the panel covers approximately 80% of the lightbox area and is drawn over the image

<!-- focused-spec:id=design-system.mobile-lightbox-scrim -->
#### Scenario: Expanded panel renders a scrim over the image

- **WHEN** the detail panel is expanded on a mobile viewport
- **THEN** a scrim is rendered over the image area that is not covered by the panel

### Requirement: Expanded mobile lightbox panel can be dismissed without the toggle

When expanded on a viewport below the `md` breakpoint, the lightbox detail panel SHALL close when the user taps the image area that remains visible beside the panel.

<!-- focused-spec:id=design-system.mobile-lightbox-dismiss-outside -->
#### Scenario: Tapping the exposed image area closes the panel

- **WHEN** the detail panel is expanded on a mobile viewport and the user taps the visible image area beside the panel
- **THEN** the detail panel is collapsed

### Requirement: Expanded mobile lightbox panel follows screen orientation

On a viewport below the `md` breakpoint the expanded lightbox detail panel SHALL be anchored to the bottom edge of the lightbox in portrait orientation and to the right edge in landscape orientation, and its open state SHALL survive an orientation change.

<!-- focused-spec:id=design-system.mobile-lightbox-panel-portrait-bottom -->
#### Scenario: Expanded panel is anchored to the bottom in portrait

- **WHEN** the detail panel is expanded on a mobile viewport in portrait orientation
- **THEN** the panel is anchored to the bottom edge of the lightbox

<!-- focused-spec:id=design-system.mobile-lightbox-panel-landscape-right -->
#### Scenario: Expanded panel is anchored to the right in landscape

- **WHEN** the detail panel is expanded on a mobile viewport in landscape orientation
- **THEN** the panel is anchored to the right edge of the lightbox

<!-- focused-spec:id=design-system.mobile-lightbox-rotation-persist -->
#### Scenario: Panel open state survives an orientation change

- **WHEN** the detail panel is expanded on a mobile viewport and the orientation changes between portrait and landscape
- **THEN** the panel remains expanded and re-anchors to the new edge

### Requirement: Mobile lightbox panel toggle is accessible

The mobile lightbox panel toggle SHALL present a touch target of at least 44×44 CSS pixels, SHALL expose an accessible name in the active language (English or Russian), and SHALL expose its expanded state and the panel it controls to assistive technology. Because the toggle is the only affordance that reveals the collapsed panel, it SHALL keep a visible text label on mobile and is exempt from the icon-first collapse rule.

<!-- focused-spec:id=design-system.mobile-lightbox-toggle-accessible -->
#### Scenario: Panel toggle exposes an accessible name and a large touch target

- **WHEN** the lightbox panel toggle is rendered on a mobile viewport
- **THEN** it exposes an accessible name in the active language and a touch target of at least 44×44 CSS pixels

<!-- focused-spec:id=design-system.mobile-lightbox-toggle-aria-expanded -->
#### Scenario: Panel toggle exposes its expanded state

- **WHEN** the lightbox panel toggle is rendered on a mobile viewport
- **THEN** it exposes an expanded/collapsed state and references the panel it controls

<!-- focused-spec:id=design-system.mobile-lightbox-toggle-visible-label -->
#### Scenario: Collapsed toggle keeps a visible label

- **WHEN** the lightbox panel is collapsed on a mobile viewport and the toggle is rendered
- **THEN** the toggle shows a visible text label in the active language rather than an icon alone

### Requirement: Mobile lightbox panel stays usable with the on-screen keyboard

On a viewport below the `md` breakpoint, when the on-screen keyboard is shown while the lightbox detail panel is expanded, the panel's text input SHALL remain visible above the keyboard.

<!-- focused-spec:id=design-system.mobile-lightbox-keyboard-visible -->
#### Scenario: Expanded panel keeps its input above the keyboard

- **WHEN** the lightbox detail panel is expanded on a mobile viewport and the on-screen keyboard is shown while a panel text input has focus
- **THEN** the panel's text input remains visible above the keyboard

### Requirement: Mobile lightbox panel respects safe-area insets

The expanded lightbox detail panel on a viewport below the `md` breakpoint SHALL keep its content and controls clear of the device safe-area insets (notch and home indicator).

<!-- focused-spec:id=design-system.mobile-lightbox-safe-area -->
#### Scenario: Expanded panel avoids the device safe areas

- **WHEN** the detail panel is expanded on a mobile viewport that has safe-area insets
- **THEN** the panel content and toggle control are inset from the safe-area edges

### Requirement: Mobile lightbox panel motion respects reduced-motion

The lightbox detail panel's expand/collapse transition on a viewport below the `md` breakpoint SHALL be suppressed when the user prefers reduced motion.

<!-- focused-spec:id=design-system.mobile-lightbox-reduced-motion -->
#### Scenario: Panel transition is suppressed under reduced motion

- **WHEN** the detail panel expands or collapses on a mobile viewport and the user prefers reduced motion
- **THEN** the panel changes state without a transition animation

### Requirement: Mobile lightbox panel manages keyboard focus

When the lightbox detail panel is expanded or collapsed on a viewport below the `md` breakpoint, focus SHALL move into the panel on expand and return to the toggle control on collapse.

<!-- focused-spec:id=design-system.mobile-lightbox-focus-management -->
#### Scenario: Focus moves into the panel and back to the toggle

- **WHEN** the detail panel is expanded on a mobile viewport and then collapsed
- **THEN** focus moves into the panel on expand and returns to the toggle control on collapse

### Requirement: Mobile lightbox panel closes before the lightbox on back

On a viewport below the `md` breakpoint, when the detail panel is expanded and the user triggers the back gesture (Escape or the platform back action), the panel SHALL collapse before the lightbox itself closes.

<!-- focused-spec:id=design-system.mobile-lightbox-back-dismiss -->
#### Scenario: Back closes the expanded panel first

- **WHEN** the detail panel is expanded on a mobile viewport and the user triggers the back gesture
- **THEN** the panel collapses and the lightbox stays open

### Requirement: Lightbox controls follow the mobile responsive rules

On a viewport below the `md` breakpoint the lightbox chrome — its mode tab bar, action controls, and informational readouts — SHALL follow the design system's existing mobile responsive rules for layout, icon-first actions, touch targets, and informational labels, and any popup that a collapsed informational readout reveals SHALL open inside the lightbox width rather than overflowing the lightbox edge.

<!-- focused-spec:id=design-system.mobile-lightbox-tabs-wrap -->
#### Scenario: Lightbox tab bar wraps without horizontal overflow

- **WHEN** a lightbox below the `md` breakpoint shows a mode tab bar with multiple tabs
- **THEN** the tabs wrap or scroll within the lightbox width without causing horizontal overflow

<!-- focused-spec:id=design-system.mobile-lightbox-icon-first-actions -->
#### Scenario: Lightbox action labels collapse to icons

- **WHEN** a lightbox action control has an informative icon together with a text label below the `md` breakpoint
- **THEN** the text label is hidden while the icon and an accessible name remain available

<!-- focused-spec:id=design-system.mobile-lightbox-chrome-touch-targets -->
#### Scenario: Lightbox chrome controls meet the touch target

- **WHEN** a lightbox chrome control (for example the dialog close button) is rendered below the `md` breakpoint
- **THEN** it presents a touch target of at least 44×44 CSS pixels

<!-- focused-spec:id=design-system.mobile-lightbox-info-label-collapse -->
#### Scenario: Long lightbox informational labels collapse to an info icon

- **WHEN** the lightbox below the `md` breakpoint shows a long informational label such as a token or processing-time readout
- **THEN** the long label is replaced by an icon-only control whose popup reveals the full text

<!-- focused-spec:id=design-system.mobile-lightbox-info-popup-contained -->
#### Scenario: Info popup opens inside the lightbox

- **WHEN** the icon control of a collapsed long informational readout is activated below the `md` breakpoint
- **THEN** the popup opens inside the lightbox width instead of overflowing the lightbox edge

<!-- focused-spec:id=design-system.mobile-lightbox-compare-actions -->
#### Scenario: Compare-slider actions collapse to icons with touch targets

- **WHEN** the enhancement compare-slider view is shown below the `md` breakpoint with its action controls (replace, save copy, reject)
- **THEN** each action hides its text label, keeps its icon and accessible name, and presents a touch target of at least 44×44 CSS pixels

### Requirement: Lightbox exposes a single close control

A lightbox SHALL render exactly one close control at every viewport width, and that control SHALL be drawn above the lightbox detail panel whenever the panel is shown.

<!-- focused-spec:id=design-system.mobile-lightbox-single-close-control -->
#### Scenario: Lightbox renders a single close control

- **WHEN** a lightbox is rendered at any viewport width
- **THEN** exactly one close control is present in the lightbox

<!-- focused-spec:id=design-system.mobile-lightbox-close-above-panel -->
#### Scenario: Close control is drawn above the detail panel

- **WHEN** the lightbox is shown with its detail panel at or above the `md` breakpoint
- **THEN** the single close control is drawn above the panel and remains visible

### Requirement: Desktop lightbox layout is unaffected by the mobile rules

At the `md` breakpoint and above a lightbox SHALL keep its pre-existing side-by-side layout — the detail panel as a fixed-width column beside the image area and the mode tab bar on a single row — and the mobile overlay and orientation rules SHALL NOT apply at that width.

<!-- focused-spec:id=design-system.mobile-lightbox-desktop-column-ratio -->
#### Scenario: Detail panel keeps its fixed-width column at md and above

- **WHEN** a lightbox with a detail panel is shown at the `md` breakpoint or above
- **THEN** the detail panel keeps its fixed-width column and the mobile overlay and orientation sizing do not apply at that width

<!-- focused-spec:id=design-system.mobile-lightbox-desktop-tabs-single-row -->
#### Scenario: Tab bar stays on a single row at md and above

- **WHEN** the lightbox mode tab bar is shown at the `md` breakpoint or above
- **THEN** the tabs stay on a single row instead of wrapping

### Requirement: Trash lightbox header follows the mobile rules

On a viewport below the `md` breakpoint the trash lightbox title bar SHALL truncate long file names instead of overflowing, and its chrome controls SHALL present a touch target of at least 44×44 CSS pixels.

<!-- focused-spec:id=design-system.mobile-lightbox-trash-header -->
#### Scenario: Trash lightbox title truncates and its controls meet the touch target

- **WHEN** the trash lightbox is shown below the `md` breakpoint with a long file name
- **THEN** the title truncates rather than overflowing and its chrome controls present a touch target of at least 44×44 CSS pixels
