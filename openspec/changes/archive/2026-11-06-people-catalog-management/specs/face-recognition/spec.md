# Spec Delta — face-recognition

## ADDED Requirements

### Requirement: A person's name can be set or cleared

`PATCH /api/people/:id` SHALL set the person's display name from the request's
`name`, trimming surrounding whitespace. An empty or whitespace-only name SHALL
clear the name so the person is unnamed again. A name longer than 200 characters
SHALL be rejected. The response SHALL be the updated person.

<!-- focused-spec:id=face-recognition.person-renamed -->
#### Scenario: A person is renamed

- **WHEN** a client sends `PATCH /api/people/7` with the name "Василиса"
- **THEN** the response reports person 7 with the name "Василиса" and `GET /api/people` lists that same name

<!-- focused-spec:id=face-recognition.person-name-trimmed -->
#### Scenario: A padded name is trimmed

- **WHEN** a client renames a person with the name "  Anna  "
- **THEN** the stored and returned name is "Anna"

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

<!-- focused-spec:id=face-recognition.person-name-duplicate-allowed -->
#### Scenario: Two people may share a name

- **WHEN** a client renames a second person to the name an existing person already has
- **THEN** both people keep the same name and the listing still shows them as separate people

### Requirement: People can be merged into one person

`POST /api/people/merge` SHALL merge every person in `sourcePersonIds` into
`targetPersonId`: their faces SHALL reference the target, the source people
SHALL no longer be listed, and the target's photo count SHALL count each photo
once. A `name` in the request SHALL become the merged person's name; without it
the target keeps its name.

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

### Requirement: A person can be created from unassigned faces

`POST /api/people` SHALL create a person, optionally named, and assign every
face in `faceIds` to it. `faceIds` SHALL be non-empty, and every face SHALL
currently be unassigned. The created person SHALL appear in `GET /api/people`
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

### Requirement: A cluster can be disbanded into unassigned faces

`DELETE /api/people/:id` SHALL make every face of that person unassigned
(`personId` 0) and remove the person. Photos and face records SHALL NOT be
deleted.

<!-- focused-spec:id=face-recognition.person-disbanded -->
#### Scenario: Disbanding returns the faces to unassigned

- **WHEN** a client deletes person 5, whose three faces appear on two photos
- **THEN** those three faces report `personId` 0, person 5 is no longer listed, and both photos still exist

<!-- focused-spec:id=face-recognition.disband-unknown-rejected -->
#### Scenario: Disbanding an unknown person changes nothing

- **WHEN** a client deletes a person that does not exist
- **THEN** the response is 404 and no face changes its person

### Requirement: Unassigned faces are listed with their photo and box

`GET /api/faces/unassigned` SHALL return the faces that belong to no person,
paginated, each with the path of its photo, its box in the analysed image's
pixel frame, and its detection confidence. Faces on photos that no longer exist
SHALL be excluded.

<!-- focused-spec:id=face-recognition.unassigned-faces-listed -->
#### Scenario: Only unassigned faces are returned

- **WHEN** the gallery holds three unassigned faces and two faces assigned to a person
- **THEN** the response contains exactly those three faces, each with a photo path, a box and a confidence

<!-- focused-spec:id=face-recognition.unassigned-faces-paginated -->
#### Scenario: Unassigned faces are paginated

- **WHEN** more unassigned faces exist than one page holds
- **THEN** the first page reports `hasNextPage: true` and the next page returns different faces

<!-- focused-spec:id=face-recognition.unassigned-faces-empty -->
#### Scenario: No unassigned faces returns an empty list

- **WHEN** every detected face belongs to a person
- **THEN** the response contains an empty list of faces

### Requirement: A face can be assigned to or detached from a person

`PATCH /api/faces/:id` SHALL set the face's person from `personId`, where
`personId` 0 SHALL detach the face. An unknown face or person SHALL be rejected
without changing anything.

<!-- focused-spec:id=face-recognition.face-joins-person -->
#### Scenario: A face joins a person

- **WHEN** a client assigns unassigned face 42 to person 7
- **THEN** face 42 reports person 7, appears in that person's photo list, and is no longer listed as unassigned

<!-- focused-spec:id=face-recognition.face-leaves-person -->
#### Scenario: A face leaves its person

- **WHEN** a client assigns `personId` 0 to a face of person 7
- **THEN** that face reports `personId` 0 and is listed among the unassigned faces

<!-- focused-spec:id=face-recognition.face-assign-unknown-rejected -->
#### Scenario: Assigning to an unknown person is rejected

- **WHEN** a client assigns a face to a person that does not exist
- **THEN** the response is 404 and the face keeps its previous person

### Requirement: Failed catalog mutations leave the catalog unchanged

Every catalog mutation that fails SHALL leave the stored people and faces
unchanged and SHALL surface an error to the user instead of silently changing
the view.

<!-- focused-spec:id=face-recognition.catalog-failure-leaves-view-unchanged -->
#### Scenario: A failed mutation keeps the view intact

- **WHEN** a rename, merge, disband or face assignment fails on the server
- **THEN** the People tab or lightbox keeps showing the previous people and faces and reports the failure

### Requirement: The workspace People tab renames a person in place

The People tab SHALL offer a rename action on every person tile and on the
person detail header. Saving SHALL update the displayed name without a page
reload, and cancelling SHALL leave the name unchanged.

<!-- focused-spec:id=face-recognition.people-tab-renames-person -->
#### Scenario: A tile shows the new name after renaming

- **WHEN** the user renames a tile labelled "Person 12" to "Василиса" and saves
- **THEN** that tile and the person detail header show "Василиса"

<!-- focused-spec:id=face-recognition.people-tab-rename-cancel -->
#### Scenario: Cancelling a rename keeps the old name

- **WHEN** the user opens the rename action, types another name and cancels
- **THEN** the person keeps its previous name and no request is sent

### Requirement: The workspace People tab merges selected people

The People tab SHALL let the user select two or more people and merge them, and
SHALL show the resulting name and the number of distinct photos before the
merge is confirmed.

<!-- focused-spec:id=face-recognition.people-tab-merges-selected -->
#### Scenario: Selected people become one tile

- **WHEN** the user selects "Person 1", "Person 2" and "Person 3", merges them with the name "Василиса" and confirms
- **THEN** the grid shows a single "Василиса" tile whose photo count covers the photos of all three

<!-- focused-spec:id=face-recognition.people-tab-merge-needs-two -->
#### Scenario: Merge stays unavailable for a single selection

- **WHEN** the user has selected exactly one person
- **THEN** the merge action is unavailable

### Requirement: The workspace People tab confirms disbanding a cluster

The People tab SHALL ask for confirmation before disbanding a person, stating
that the photos stay and the faces return to unassigned, and SHALL disband the
person only after the confirmation.

<!-- focused-spec:id=face-recognition.people-tab-confirms-disband -->
#### Scenario: Confirmed disbanding removes the tile

- **WHEN** the user chooses to disband a person and confirms
- **THEN** the tile disappears from the grid and the person's faces are listed among the unassigned faces

<!-- focused-spec:id=face-recognition.people-tab-disband-cancel -->
#### Scenario: Cancelling keeps the person

- **WHEN** the user opens the disband confirmation and cancels
- **THEN** the person and its photos stay unchanged

### Requirement: The workspace People tab filters people by name

The People tab SHALL provide a search field that filters the displayed people by
their display name, case-insensitively, and matches unnamed people by their
localized placeholder.

<!-- focused-spec:id=face-recognition.people-tab-filters-by-name -->
#### Scenario: Typing a name narrows the grid

- **WHEN** the user types "вас" in the search field
- **THEN** only the people whose display name contains "вас" in any case remain visible

<!-- focused-spec:id=face-recognition.people-tab-filter-no-match -->
#### Scenario: A filter without matches says so

- **WHEN** the search text matches no person
- **THEN** the tab shows a no-matches message instead of the empty-catalog state

<!-- focused-spec:id=face-recognition.people-tab-filter-matches-placeholder -->
#### Scenario: Searching the placeholder finds unnamed people

- **WHEN** the user types part of the localized "Person N" placeholder in the search field
- **THEN** the unnamed people whose placeholder contains that text remain visible

### Requirement: The workspace People tab manages unassigned faces

The People tab SHALL expose an Unassigned faces view that shows each unassigned
face as its photo thumbnail with the face box, and SHALL let the user select
faces and either attach them to an existing person or create a new named person
from them.

<!-- focused-spec:id=face-recognition.unassigned-view-lists-faces -->
#### Scenario: The unassigned view shows each face on its photo

- **WHEN** the user opens the Unassigned faces view with two unassigned faces
- **THEN** it shows two entries, each a photo thumbnail with the corresponding face box

<!-- focused-spec:id=face-recognition.unassigned-face-attached -->
#### Scenario: A selected face joins an existing person

- **WHEN** the user selects an unassigned face, chooses person "Василиса" and attaches it
- **THEN** the face disappears from the unassigned view and "Василиса" gains that photo

<!-- focused-spec:id=face-recognition.unassigned-faces-create-person -->
#### Scenario: Selected faces become a new person

- **WHEN** the user selects two unassigned faces, creates a person named "Василиса" from them and confirms
- **THEN** the grid shows a "Василиса" tile with those faces and the faces leave the unassigned view

<!-- focused-spec:id=face-recognition.unassigned-view-pages -->
#### Scenario: The unassigned view pages through faces

- **WHEN** more unassigned faces exist than one page shows
- **THEN** the view offers to load the next page and shows different faces when it is loaded

### Requirement: The lightbox People tab renames a person

In the lightbox People tab, each entry of the people list SHALL offer a rename
action that updates that person's name in the list and in the person's labels.

<!-- focused-spec:id=face-recognition.lightbox-renames-person -->
#### Scenario: A person is renamed from the photo

- **WHEN** the user renames "Person 3" to "Василиса" in the lightbox people list and saves
- **THEN** the list entry shows "Василиса" and the People grid shows that name afterwards

### Requirement: The lightbox People tab reassigns or detaches a face

In the lightbox People tab, selecting a face box SHALL offer assigning that face
to any person in the catalog (not only the people already on the photo) or
detaching it from its person, and the boxes, numbers and people list SHALL
reflect the new assignment.

<!-- focused-spec:id=face-recognition.lightbox-reassigns-face -->
#### Scenario: A misclassified face moves to another person

- **WHEN** the user selects a face numbered for "Anna" and assigns it to "Василиса"
- **THEN** that face carries Василиса's number and Anna's entry loses that face

<!-- focused-spec:id=face-recognition.lightbox-detaches-face -->
#### Scenario: A face can be detached from its person

- **WHEN** the user selects a numbered face and chooses that it is not that person
- **THEN** the box loses its number and is drawn muted, and the face is listed among the unassigned faces

<!-- focused-spec:id=face-recognition.lightbox-reassign-compacts-list -->
#### Scenario: A person with no remaining face disappears from the list

- **WHEN** the reassigned face was that person's only face on the photo
- **THEN** the person's entry disappears and the remaining people are renumbered without gaps

<!-- focused-spec:id=face-recognition.lightbox-reassign-picker-lists-whole-catalog -->
#### Scenario: The reassignment picker lists people not on the photo

- **WHEN** the user reassigns a face and the person they want is not present on the photo
- **THEN** that person is still offered in the picker and can be chosen

### Requirement: The lightbox People tab attaches an unassigned face

In the lightbox People tab, selecting a face that belongs to no person SHALL
offer assigning it to an existing person or to a new person, after which the box
carries that person's number.

<!-- focused-spec:id=face-recognition.lightbox-attaches-unassigned-face -->
#### Scenario: An unnumbered face joins a person

- **WHEN** the user selects an unnumbered face on the photo and assigns it to person "Василиса"
- **THEN** that box carries Василиса's number and the people list includes Василиса

<!-- focused-spec:id=face-recognition.lightbox-attaches-to-new-person -->
#### Scenario: An unnumbered face becomes a new person

- **WHEN** the user selects an unnumbered face and creates a new named person from it in the lightbox
- **THEN** the box carries the new person's number, the people list includes it, and the People grid shows it afterwards

### Requirement: Catalog management strings are localized

Every user-facing string introduced by catalog management SHALL exist in both
English and Russian.

<!-- focused-spec:id=face-recognition.catalog-strings-localized -->
#### Scenario: Catalog actions are localized

- **WHEN** the user switches the interface language between English and Russian and opens the People tab
- **THEN** the rename, merge, disband and unassigned-faces labels render in the selected language without falling back to a raw key
