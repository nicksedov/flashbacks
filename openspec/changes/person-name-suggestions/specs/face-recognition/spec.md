# Spec Delta

## ADDED Requirements

### Requirement: The lightbox rename dialog suggests catalog names

In the lightbox People tab, the rename dialog SHALL show suggestion chips under
the name input as soon as it opens: the named people with the most photos, at
most seven, even when the field starts with the person's current name. Once the
user edits the field, only names containing the typed fragment SHALL be offered.
The renamed person and unnamed people SHALL never be suggested.

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestions-before-typing -->
#### Scenario: An unnamed person's dialog opens with the most-photographed names

- **WHEN** the user opens the rename dialog on an unnamed person and the catalog holds more than seven named people
- **THEN** the dialog shows exactly seven suggestions, ordered by photo count with the most photos first

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestions-prefilled-name -->
#### Scenario: A prefilled name does not hide the suggestions

- **WHEN** the user opens the rename dialog on a person already named "Анна"
- **THEN** the dialog shows the same seven most-photographed names instead of an empty list

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestions-ties -->
#### Scenario: Equal photo counts are ordered by name

- **WHEN** two suggested people have the same photo count
- **THEN** their entries appear in name order so the list is stable between openings

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestions-filter -->
#### Scenario: Editing the field narrows the suggestions to names containing the fragment

- **WHEN** the user edits the name field and types "вас"
- **THEN** the dialog shows only the people whose name contains "вас", at most seven, most photos first

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestions-ignores-case -->
#### Scenario: A fragment matches regardless of case and surrounding spaces

- **WHEN** the user types " ИВАН " and a person is named "Иван Петров"
- **THEN** that person is offered as a suggestion

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestions-no-match -->
#### Scenario: A fragment matching no name shows no suggestions

- **WHEN** the user types a fragment that matches no catalog name
- **THEN** no suggestion entries are shown and the input stays usable

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestions-exclude-renamed -->
#### Scenario: The person being renamed is never suggested

- **WHEN** the user renames a person already named "Иван" and types "иван"
- **THEN** that same person is not offered among the suggestions

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestions-skip-unnamed -->
#### Scenario: Unnamed people are never suggested

- **WHEN** the catalog contains people with no name and the rename dialog is open
- **THEN** those people never appear among the suggestions, whether the input is edited or not

### Requirement: A chosen suggestion fills the name without saving

Choosing a suggestion SHALL put that name into the name input and SHALL NOT save
or close the dialog; the user still submits with Save. Because a suggestion is by
definition a name another person already holds, saving it SHALL reach the
existing conflict dialog, which offers merging into the person that holds it, and
SHALL never create a second person with that name.

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestion-fills-input -->
#### Scenario: Clicking a suggestion fills the input only

- **WHEN** the user clicks the suggestion "Иван Петров"
- **THEN** the name field holds "Иван Петров", the dialog stays open and no rename request is sent until Save is pressed

<!-- focused-spec:id=face-recognition.lightbox-rename-suggestion-opens-conflict -->
#### Scenario: Saving a suggested name offers the merge into its holder

- **WHEN** the user picks the suggestion "Иван", saves, and the dialog names the person that already holds "Иван"
- **THEN** no second "Иван" is created and the conflict dialog offers merging the renamed person into that holder
