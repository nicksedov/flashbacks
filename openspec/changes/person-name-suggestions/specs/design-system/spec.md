# Spec Delta

## ADDED Requirements

### Requirement: Inline suggestions share one chip primitive

Every inline suggestion list — the AI assistant's prompt suggestions and the
person-name suggestions in the rename dialog — SHALL render each entry through
the same shared `SuggestionChip` primitive, so a suggestion looks and behaves
identically wherever it appears. Entries SHALL show only the suggestion's own
text: no list heading, no secondary counter, and no per-surface variant.

<!-- focused-spec:id=design-system.suggestion-chips-uniform -->
#### Scenario: Assistant and rename suggestions look alike

- **WHEN** the user compares the AI assistant's prompt suggestions with the name suggestions in the lightbox rename dialog
- **THEN** both render as the same pill chip with the same border, type scale and hover fill

<!-- focused-spec:id=design-system.suggestion-chips-text-only -->
#### Scenario: A suggestion chip carries only its label

- **WHEN** a surface renders suggestions
- **THEN** each chip shows only the suggestion text, with no group heading and no secondary count beside it
