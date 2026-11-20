# Spec Delta — face-recognition

## REMOVED Requirements

### Requirement: A person's name can be set or cleared

**Reason**: this requirement asserted that two people may share a name
(`person-name-duplicate-allowed`), which is the defect being fixed. It is
replaced by `A person's name can be set or cleared uniquely`, which keeps every
other behavior of the old requirement and adds the uniqueness rule.

**Migration**: the endpoint and its payload are unchanged — clients keep sending
`PATCH /api/people/:id` with `{ "name": string }`. A name that another person
already holds now answers `409` with the error key `people.name_taken` and a
`conflict` object describing that person; the UI resolves it by merging into the
holder or by entering a different name.

## MODIFIED Requirements

### Requirement: People can be merged into one person

`POST /api/people/merge` SHALL merge every person in `sourcePersonIds` into
`targetPersonId`: their faces SHALL reference the target, the source people
SHALL no longer be listed, and the target's photo count SHALL count each photo
once. A `name` in the request SHALL become the merged person's name; without it
the target keeps its name.

A `name` SHALL be subject to the same uniqueness rule as a rename, but only
against people outside the merge: the target's own name and the names of the
sources being folded in SHALL NOT count as conflicts. When the requested name
belongs to another person, the request SHALL fail with `409` and the error key
`people.name_taken`, and no person SHALL be modified.

<!-- focused-spec:id=face-recognition.people-merged -->
#### Scenario: Three clusters become one person

- **WHEN** a client merges people 2 and 3 into target person 1
- **THEN** every face of people 2 and 3 references person 1 and `GET /api/people` no longer lists people 2 and 3

<!-- focused-spec:id=face-recognition.merge-applies-name -->
#### Scenario: The merge sets the resulting name

- **WHEN** a client merges people 1 and 2 with the name "Василиса"
- **THEN** the merged person is listed with the name "Василиса"

<!-- focused-spec:id=face-recognition.merge-keeps-target-name -->
#### Scenario: A merge without a name keeps the target's name

- **WHEN** a client merges a named person into another person without sending a name
- **THEN** the target keeps the name it had before the merge

<!-- focused-spec:id=face-recognition.merge-counts-photo-once -->
#### Scenario: A photo showing both merged people counts once

- **WHEN** a photo contains a face of person 1 and a face of person 2 and the two people are merged
- **THEN** the merged person's `photoCount` counts that photo once

<!-- focused-spec:id=face-recognition.merge-unknown-rejected -->
#### Scenario: A merge with an unknown person is rejected

- **WHEN** a merge request names a source person that does not exist
- **THEN** the response is 404 and no person is modified

<!-- focused-spec:id=face-recognition.merge-rejects-empty-sources -->
#### Scenario: A merge without sources is rejected

- **WHEN** a merge request sends an empty `sourcePersonIds`
- **THEN** the response is 400 and no person changes

<!-- focused-spec:id=face-recognition.merge-rejects-target-in-sources -->
#### Scenario: A merge into one of its own sources is rejected

- **WHEN** a merge request lists the target person among `sourcePersonIds`
- **THEN** the response is 400 and no person changes

<!-- focused-spec:id=face-recognition.merge-name-taken-rejected -->
#### Scenario: A merge name held by an outside person is refused

- **WHEN** a client merges person 1 and 2 with the name "Иван" while person 5, which is not part of the merge, is named "Иван"
- **THEN** the response is 409 with the error key `people.name_taken`, no person is merged, and every face keeps its person

<!-- focused-spec:id=face-recognition.merge-name-of-target-allowed -->
#### Scenario: A merge keeps the target name it already holds

- **WHEN** person 1 is named "Иван" and a client merges person 2 into person 1 with the name "Иван"
- **THEN** the merge succeeds and the result is the single person named "Иван"

<!-- focused-spec:id=face-recognition.merge-name-of-source-allowed -->
#### Scenario: A source name does not block its own merge

- **WHEN** person 2 is named "Иван" and a client merges person 2 into the unnamed person 1 with the name "Иван"
- **THEN** the merge succeeds and the result is the single person named "Иван"

### Requirement: A person can be created from unassigned faces

`POST /api/people` SHALL create a person, optionally named, and assign every
face in `faceIds` to it. `faceIds` SHALL be non-empty, and every face SHALL
currently be unassigned. A name SHALL be subject to the same uniqueness rule as a
rename: when it belongs to another person the request SHALL fail with `409` and
the error key `people.name_taken`, and no person SHALL be created and no face
SHALL change its person. The created person SHALL appear in `GET /api/people`
with its photo count.

<!-- focused-spec:id=face-recognition.person-created-from-faces -->
#### Scenario: A new person is built from unassigned faces

- **WHEN** a client creates a person named "Василиса" from two unassigned faces
- **THEN** both faces reference the new person and the listing shows that person

<!-- focused-spec:id=face-recognition.person-created-without-name -->
#### Scenario: A person can be created unnamed

- **WHEN** a client creates a person without a name from one unassigned face
- **THEN** the listing shows the new person with an empty name

<!-- focused-spec:id=face-recognition.person-create-rejects-empty-faces -->
#### Scenario: Creating a person without faces is rejected

- **WHEN** a client creates a person with an empty `faceIds`
- **THEN** the response is 400 and no person is created

<!-- focused-spec:id=face-recognition.person-create-unknown-face-rejected -->
#### Scenario: Creating a person from an unknown or assigned face is rejected

- **WHEN** a client creates a person from a face that does not exist or already belongs to a person
- **THEN** the response is 404 and no person is created

<!-- focused-spec:id=face-recognition.person-create-name-taken-rejected -->
#### Scenario: Creating a person with a taken name is refused

- **WHEN** a client creates a person named "Иван" from two unassigned faces while person 3 is named "Иван"
- **THEN** the response is 409 with the error key `people.name_taken`, no person is created, and both faces stay unassigned

### Requirement: Catalog management strings are localized

Every user-facing string introduced by catalog management, including the name
conflict dialog and its two alternatives, SHALL exist in both English and
Russian.

<!-- focused-spec:id=face-recognition.catalog-strings-localized -->
#### Scenario: Catalog actions are localized

- **WHEN** the user switches the interface language between English and Russian and opens the People tab
- **THEN** the rename, merge, disband and unassigned-faces labels render in the selected language without falling back to a raw key

<!-- focused-spec:id=face-recognition.name-conflict-strings-localized -->
#### Scenario: The conflict dialog is localized

- **WHEN** the interface language is Russian and the user types a name that already exists
- **THEN** the conflict title, its explanation, the merge action and the change-name action render in Russian without falling back to a raw key

## ADDED Requirements

### Requirement: A person's name can be set or cleared uniquely

`PATCH /api/people/:id` SHALL set the person's display name from the request's
`name`, and SHALL clear it when the name is blank. Names are compared after
normalization — trimmed, internal whitespace collapsed, case-folded — and a name
already held by a **different** person SHALL be refused with `409` and the error
key `people.name_taken`, whose body carries the holder as `conflict`. A
successful response SHALL be the updated person.

<!-- focused-spec:id=face-recognition.person-renamed -->
#### Scenario: A person is renamed

- **WHEN** a client sends `PATCH /api/people/7` with the name "Василиса"
- **THEN** the response reports person 7 with the name "Василиса" and `GET /api/people` lists that same name

<!-- focused-spec:id=face-recognition.person-name-trimmed -->
#### Scenario: A padded name is trimmed

- **WHEN** a client renames a person with the name "  Anna  "
- **THEN** the stored and returned name is "Anna"

<!-- focused-spec:id=face-recognition.person-name-inner-whitespace-collapsed -->
#### Scenario: A name keeps single inner spaces

- **WHEN** a client renames a person with the name "Anna   Petrova"
- **THEN** the stored and returned name is "Anna Petrova"

<!-- focused-spec:id=face-recognition.person-name-cleared -->
#### Scenario: An empty name returns the person to unnamed

- **WHEN** a client renames a named person with an empty name
- **THEN** the response reports an empty name and the listing shows the person as unnamed

<!-- focused-spec:id=face-recognition.person-rename-unknown -->
#### Scenario: Renaming an unknown person changes nothing

- **WHEN** a client sends `PATCH /api/people/999` for a person that does not exist
- **THEN** the response is 404 and no person is created or renamed

<!-- focused-spec:id=face-recognition.person-name-too-long-rejected -->
#### Scenario: An over-long name is rejected

- **WHEN** a client renames a person with a name longer than 200 characters
- **THEN** the response is 400 and the person keeps its previous name

<!-- focused-spec:id=face-recognition.person-name-taken-rejected -->
#### Scenario: Renaming to a taken name is refused

- **WHEN** a client renames person 7 to "Иван" while person 3 is already named "Иван"
- **THEN** the response is 409, person 7 keeps its previous name, and `GET /api/people` still lists exactly one "Иван"

<!-- focused-spec:id=face-recognition.person-name-taken-reports-holder -->
#### Scenario: The refusal names the person that holds the name

- **WHEN** a client renames person 7 to the name person 3 already has
- **THEN** the 409 body carries the error key `people.name_taken` and a `conflict` object with id 3, its name, its photo count and its cover thumbnail

<!-- focused-spec:id=face-recognition.person-name-conflict-ignores-case-and-spaces -->
#### Scenario: A case or spacing variant is the same name

- **WHEN** person 3 is named "Иван" and a client renames person 7 to " иВаН "
- **THEN** the response is 409 and person 7 keeps its previous name

<!-- focused-spec:id=face-recognition.person-name-own-name-allowed -->
#### Scenario: A person may take its own name in any case

- **WHEN** person 3 is named "Иван" and a client renames person 3 to "иван"
- **THEN** the response is 200, the name is stored as "иван", and no conflict is reported

<!-- focused-spec:id=face-recognition.person-name-unnamed-never-conflicts -->
#### Scenario: Unnamed people never collide

- **WHEN** two people are unnamed and a client clears the name of a third person
- **THEN** the response is 200 and the catalog holds three unnamed people

### Requirement: Person names are unique across the catalog

At most one person SHALL hold a normalized name at any time. Every name entry
point — rename, merge and create — SHALL refuse a duplicate, and the stored
catalog SHALL keep the rule even for a write that bypasses those entry points.
People with an empty name are exempt.

<!-- focused-spec:id=face-recognition.unique-name-across-entry-points -->
#### Scenario: Every name entry point refuses a duplicate

- **WHEN** person 3 is named "Иван" and the name "Иван" is submitted through rename, through merge and through create
- **THEN** all three requests fail without changing any person or face

<!-- focused-spec:id=face-recognition.unique-name-storage-guarantee -->
#### Scenario: The stored catalog cannot hold two equal names

- **WHEN** a duplicate normalized name is written straight to the people table, bypassing the API
- **THEN** the write is rejected and the catalog still holds one person with that name

### Requirement: Legacy duplicate names are reconciled at startup

At startup the service SHALL reduce every group of people whose names are equal
after normalization to one person: the lowest id keeps the name and absorbs the
others' faces, cover and exemplars. The repair SHALL be idempotent and SHALL
leave unnamed people untouched.

<!-- focused-spec:id=face-recognition.startup-duplicates-merged -->
#### Scenario: Two legacy people with one name become one person

- **WHEN** the service starts on a database whose people 20 and 35 are both named "Иван" and both hold faces
- **THEN** after startup person 20 is the only "Иван", the faces of person 35 reference person 20, and the photos of both appear under person 20

<!-- focused-spec:id=face-recognition.startup-repair-idempotent -->
#### Scenario: A clean catalog is left alone

- **WHEN** the service restarts on a catalog whose names are already unique
- **THEN** no person, face or exemplar changes and no name is cleared

<!-- focused-spec:id=face-recognition.startup-repair-keeps-unnamed -->
#### Scenario: Unnamed people survive the repair

- **WHEN** the service starts on a catalog holding several unnamed people
- **THEN** every unnamed person keeps its own identity and none of them is merged

### Requirement: A taken name offers merge or another name

When a name submitted from the UI is already taken, the client SHALL NOT save
it. It SHALL show a conflict dialog naming the holder and offering two
alternatives: merge the pending person, selection or faces into the holder, or
return to the input for a different name. Cancelling SHALL change nothing.

<!-- focused-spec:id=face-recognition.conflict-dialog-shows-holder -->
#### Scenario: The conflict dialog identifies the existing person

- **WHEN** the user submits the name "Иван" and the catalog already holds "Иван" with 12 photos
- **THEN** the dialog shows that person's name and photo count and does not send a rename that would create a second "Иван"

<!-- focused-spec:id=face-recognition.conflict-merge-folds-into-holder -->
#### Scenario: Merging from the conflict dialog joins the existing person

- **WHEN** the user chooses to merge in the conflict dialog
- **THEN** the pending person's faces and photos belong to the existing person, the pending person disappears, and exactly one person carries that name

<!-- focused-spec:id=face-recognition.conflict-change-name-saves-unique -->
#### Scenario: Choosing another name saves it

- **WHEN** the user chooses to enter another name and saves "Иван Петров"
- **THEN** the name is saved for the pending person and no merge happens

<!-- focused-spec:id=face-recognition.conflict-cancel-keeps-catalog -->
#### Scenario: Cancelling the dialog changes nothing

- **WHEN** the user closes the conflict dialog without choosing an alternative
- **THEN** no request changes the catalog and the person keeps its previous name

<!-- focused-spec:id=face-recognition.conflict-detected-before-request -->
#### Scenario: A name already in the loaded catalog is caught locally

- **WHEN** the user types a name that matches a person already listed in the loaded catalog
- **THEN** the conflict dialog appears without a round trip that would create a duplicate

### Requirement: The lightbox resolves a person name conflict

In the unified lightbox People tab, renaming a person to a name that already
exists SHALL open the conflict dialog and SHALL never produce a second person
with that name. Merging from the dialog SHALL fold the renamed person into the
existing one, after which the photo's boxes, numbers and people list reference
the existing person.

<!-- focused-spec:id=face-recognition.lightbox-rename-conflict-no-duplicate -->
#### Scenario: A conflicting rename in the lightbox creates no duplicate

- **WHEN** the user renames "Person 3" to "Василиса" while another person is already named "Василиса" and then cancels the conflict dialog
- **THEN** the People grid still lists exactly one "Василиса" and "Person 3" keeps its previous name

<!-- focused-spec:id=face-recognition.lightbox-conflict-merges-person -->
#### Scenario: Merging from the lightbox folds the cluster into the holder

- **WHEN** the user renames "Person 3" to "Василиса", chooses merge in the conflict dialog, and "Василиса" already holds 12 photos
- **THEN** the faces of "Person 3" now belong to "Василиса", that person's photo count grows, and the lightbox list shows one "Василиса" entry

<!-- focused-spec:id=face-recognition.lightbox-create-from-face-conflict -->
#### Scenario: Creating a person from a face with a taken name is refused

- **WHEN** the user creates a person named "Василиса" from an unassigned face while "Василиса" already exists and cancels the conflict dialog
- **THEN** no person is created and the face stays unassigned

### Requirement: The workspace People tab resolves name conflicts

The People tab SHALL resolve a taken name through the same conflict dialog on
every path that can name a person: renaming a tile or a detail header, creating
a person from selected unassigned faces, and merging a selection with a new
name. Merging with a taken name SHALL fold the whole selection into the person
that holds it, and creating from faces with a taken name SHALL offer to add those
faces to the person that holds it.

<!-- focused-spec:id=face-recognition.people-tab-rename-conflict -->
#### Scenario: A conflicting tile rename waits for the user

- **WHEN** the user renames a tile to a name another person already has
- **THEN** the conflict dialog opens, and the grid is unchanged until the user merges or chooses another name

<!-- focused-spec:id=face-recognition.people-tab-create-conflict-attaches-faces -->
#### Scenario: Creating with a taken name can attach the faces instead

- **WHEN** the user creates a person named "Василиса" from selected unassigned faces, the name is taken, and the user chooses to add the faces to that person
- **THEN** those faces belong to the existing "Василиса", no new person is created, and the faces leave the unassigned view

<!-- focused-spec:id=face-recognition.people-tab-merge-name-conflict -->
#### Scenario: Merging with a taken name folds everything into the holder

- **WHEN** the user merges two selected people with the name of a third person and chooses to continue
- **THEN** all three become the single person that held the name, and the grid shows one tile for it
