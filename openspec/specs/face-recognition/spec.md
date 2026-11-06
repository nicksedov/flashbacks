# face-recognition Specification

## Purpose

Lets users browse the photo library by the people in it: a background job
detects faces on gallery photos, clusters them into unnamed people, and a
People view lists those people and the photos each one appears in.

## Requirements

### Requirement: Faces are grouped into people by the background job

The core service SHALL run a background face recognition job that detects faces
on gallery photos and assigns every detected face to exactly one person
(cluster). An identity SHALL be stable state rather than a per-pass result: a
face analysed in a later pass SHALL join the person that already holds its
identity instead of creating a new person, and people whose faces belong to one
identity SHALL be reconciled into a single person. A face whose best match does
not clear the calibrated similarity floor and margin SHALL stay unassigned
(`personId` 0) instead of becoming a person or joining one on a weak match.

<!-- focused-spec:id=face-recognition.face-assigned -->
#### Scenario: A detected face is assigned to a person

- **WHEN** the face recognition job processes a photo that contains exactly one face
- **THEN** a face record is stored and it references exactly one person

<!-- focused-spec:id=face-recognition.same-person-grouped -->
#### Scenario: The same face across photos shares one person

- **WHEN** the face recognition job processes two photos that show the same person's face
- **THEN** both face records reference the same person

<!-- focused-spec:id=face-recognition.no-face-skipped -->
#### Scenario: A photo without a face is skipped

- **WHEN** the face recognition job processes a photo with no detectable face
- **THEN** no face record is created for that photo and the job advances to the next photo

<!-- focused-spec:id=face-recognition.identity-stable-across-passes -->
#### Scenario: A later pass joins the person of the same identity

- **WHEN** one identity is photographed twice and the two photos are analysed by two different job passes
- **THEN** both face records reference the same person

<!-- focused-spec:id=face-recognition.people-reconciled -->
#### Scenario: People holding one identity are reconciled

- **WHEN** the stored gallery holds the faces of one identity spread over several people and a clustering pass completes
- **THEN** those face records reference exactly one person

<!-- focused-spec:id=face-recognition.ambiguous-face-unassigned -->
#### Scenario: A face without a confident match stays unassigned

- **WHEN** a face's best match against every existing person is below the calibrated recognition floor
- **THEN** that face references no person

<!-- focused-spec:id=face-recognition.distinct-identities-stay-separate -->
#### Scenario: Different people stay different people

- **WHEN** the job processes photos of two different people whose faces are below the calibrated similarity floor of each other
- **THEN** their face records reference different people

### Requirement: Detected faces of one image are exposed with their person

`GET /api/image-faces?path=<imagePath>` SHALL return every face detected on that
gallery image, each with its bounding box in the analysed image's pixel frame, its
detection confidence and the person it was clustered into (`personId` is 0 when the
face belongs to none).

<!-- focused-spec:id=face-recognition.image-faces-returned -->
#### Scenario: All detected faces are returned

- **WHEN** an image with N detected faces is requested
- **THEN** the response contains exactly N face entries

<!-- focused-spec:id=face-recognition.image-face-person -->
#### Scenario: A face reports the person it belongs to

- **WHEN** a detected face has been clustered into the person named "Anna"
- **THEN** that face's entry reports the person's id and the name "Anna"

<!-- focused-spec:id=face-recognition.image-face-unassigned -->
#### Scenario: A face without a person reports no person

- **WHEN** a detected face was not clustered into any person
- **THEN** that face's entry reports `personId` 0

<!-- focused-spec:id=face-recognition.image-faces-empty -->
#### Scenario: An image without faces returns an empty list

- **WHEN** an image with no detected faces is requested
- **THEN** the response contains an empty list of faces

### Requirement: Face recognition job reports structured progress

`GET /api/faces/status` SHALL report whether the face recognition job is running
together with counters of processed and total photos.

<!-- focused-spec:id=face-recognition.status-progress -->
#### Scenario: Status reports running state and counters

- **WHEN** a face recognition job is running with N photos to process and P already processed
- **THEN** the response reports `running: true`, `processed` equal to P, and `total` equal to N

### Requirement: People are listed with a cover photo and count

`GET /api/people` SHALL return the people discovered by face recognition, each
with a display name, its photo count, and a cover thumbnail reference.

<!-- focused-spec:id=face-recognition.people-listed -->
#### Scenario: Discovered people are listed with cover and count

- **WHEN** face recognition has produced at least one person that appears in K photos
- **THEN** `GET /api/people` returns that person with `photoCount` equal to K and a non-empty cover thumbnail reference

### Requirement: A person's photos are browsable

`GET /api/people/:id/photos` SHALL return the photos in which the given person
appears, paginated like the gallery listing.

<!-- focused-spec:id=face-recognition.person-photos -->
#### Scenario: A person's photos are paginated

- **WHEN** a person appears in more photos than one page holds
- **THEN** the first page reports `hasNextPage: true` and the next page returns different, non-overlapping photos of that person

### Requirement: Face recognition is configurable and launchable

The core SHALL persist face recognition settings (enabled flag and schedule) and
SHALL start the background job when the administrator launches it.

<!-- focused-spec:id=face-recognition.settings-persisted -->
#### Scenario: Saved settings come back in the settings response

- **WHEN** an administrator saves the face recognition enabled flag together with a schedule
- **THEN** the settings response reports that same enabled flag and schedule

<!-- focused-spec:id=face-recognition.job-started -->
#### Scenario: Launching face recognition starts the job

- **WHEN** an administrator launches the face recognition job while it is not running
- **THEN** the status endpoint reports the job as running

### Requirement: People view renders a person grid

The People tab SHALL render one tile per person with the person's name and cover
thumbnail.

<!-- focused-spec:id=face-recognition.people-grid -->
#### Scenario: People grid shows every person

- **WHEN** the People tab loads with one or more discovered people
- **THEN** the tab renders one tile per person, each showing the person's name and cover thumbnail

### Requirement: People view shows an empty state

The People tab SHALL show the shared empty state when no people have been
discovered yet.

<!-- focused-spec:id=face-recognition.people-empty -->
#### Scenario: No people shows the empty state

- **WHEN** the People tab loads and no person has been discovered
- **THEN** the tab shows the empty-state title and hint instead of an empty grid

### Requirement: People view strings are localized

Every user-facing People string SHALL exist in both English and Russian.

<!-- focused-spec:id=face-recognition.people-localized -->
#### Scenario: People labels are localized

- **WHEN** the user switches the interface language between English and Russian
- **THEN** the People view labels render in the selected language without falling back to a raw key

### Requirement: Box geometry follows the rendered image

In People mode each bounding box SHALL be scaled together with the displayed image,
so a box keeps covering the same face region at any rendered size.

<!-- focused-spec:id=face-recognition.image-face-box-scaled -->
#### Scenario: A box keeps covering its face after scaling

- **WHEN** an image analysed at 200×100 pixels is rendered at 400×200 pixels and its face box sits at x=50, y=25, width=40, height=20 pixels
- **THEN** the box is rendered at x=100, y=50, width=80, height=40 pixels

### Requirement: The unified lightbox exposes a People mode before Tags

The unified lightbox SHALL offer a People mode, and its tab SHALL render immediately
before the Tags tab.

<!-- focused-spec:id=face-recognition.lightbox-people-tab-order -->
#### Scenario: People tab precedes the Tags tab

- **WHEN** the lightbox tab bar renders
- **THEN** the People tab is the tab immediately before Tags

### Requirement: People mode draws a box for every detected face

In People mode the lightbox SHALL draw a bounding-box overlay on the lightbox image
stage containing one box per detected face.

<!-- focused-spec:id=face-recognition.lightbox-people-box-per-face -->
#### Scenario: Every detected face gets a box

- **WHEN** a user opens the People tab for an image with three detected faces
- **THEN** the image stage renders three bounding boxes

### Requirement: A face belonging to a person is numbered

In People mode each face clustered into a person SHALL carry that person's number,
drawn on the face's bounding box.

<!-- focused-spec:id=face-recognition.lightbox-people-face-numbered -->
#### Scenario: Faces of a person carry the person's number

- **WHEN** the People tab is opened for an image whose two faces belong to two different people
- **THEN** each of the two face boxes is labelled with its person's number

### Requirement: A face without a person is drawn unnumbered

In People mode a face that belongs to no person SHALL be drawn without a number and
visually muted relative to the numbered boxes.

<!-- focused-spec:id=face-recognition.lightbox-people-unassigned-unnumbered -->
#### Scenario: An unassigned face carries no number

- **WHEN** the People tab is opened for an image with one face that belongs to no person
- **THEN** that face's box carries no number

### Requirement: People mode lists the people present in the image

The People panel SHALL list every person present on the image exactly once, each entry
showing the number drawn on that person's bounding box and the person's display name.

<!-- focused-spec:id=face-recognition.lightbox-people-list -->
#### Scenario: Each person appears once with its number

- **WHEN** the People tab is opened for an image where two people each have a face
- **THEN** the panel lists two entries, numbered 1 and 2, matching the numbers on the boxes

### Requirement: An unnamed person falls back to a localized placeholder

A listed person whose name is empty SHALL render the localized placeholder instead of
an empty label.

<!-- focused-spec:id=face-recognition.lightbox-people-name-fallback -->
#### Scenario: An unnamed person shows the placeholder

- **WHEN** the People panel lists a person whose name is empty
- **THEN** that entry renders the localized placeholder that identifies the person

### Requirement: People mode shows an empty state without faces

The People tab SHALL render the shared empty state when the opened image has no
detected faces.

<!-- focused-spec:id=face-recognition.lightbox-people-empty -->
#### Scenario: No faces shows the empty state

- **WHEN** the People tab is opened for an image with no detected faces
- **THEN** the panel shows the empty-state title instead of a people list

### Requirement: People mode strings are localized

Every user-facing string of the People mode SHALL exist in both English and Russian.

<!-- focused-spec:id=face-recognition.lightbox-people-localized -->
#### Scenario: People mode labels are localized

- **WHEN** the interface language is Russian
- **THEN** the People tab and the People panel render Russian labels without falling back to a raw key

### Requirement: 1:1 verification on LFW reaches the 90% floor

The faces service SHALL provide an offline calibration that measures 1:1 face
verification on the Labelled Faces in the Wild (LFW) benchmark using the official
6,000-pair, 10-fold protocol, and the mean fold accuracy at the shipped
recognition threshold SHALL be at least 0.90.

<!-- focused-spec:id=face-recognition.lfw-verification-accuracy -->
#### Scenario: Mean LFW accuracy meets the floor

- **WHEN** the calibration scores the official LFW pair protocol at the shipped recognition threshold
- **THEN** it reports the ten per-fold accuracies, their mean, and that mean is at least 0.90

<!-- focused-spec:id=face-recognition.lfw-verification-below-floor -->
#### Scenario: Accuracy below the floor fails the run

- **WHEN** the mean LFW accuracy at the shipped threshold is below 0.90
- **THEN** the calibration exits with a non-zero status, prints the measured accuracy and the best candidate threshold, and leaves the configuration unchanged

### Requirement: Verification accuracy follows the official LFW protocol

A pair SHALL be accepted as the same person when the cosine similarity of its two
embeddings is at least the shipped recognition threshold. Accuracy per fold SHALL
be the share of its 600 pairs decided correctly, and the reported accuracy SHALL
be the unweighted mean of the ten folds.

<!-- focused-spec:id=face-recognition.lfw-undetected-pair -->
#### Scenario: A pair with an undetectable face counts as a non-match

- **WHEN** either image of a pair yields no detectable face
- **THEN** the pair is scored as a non-match instead of being excluded from the accuracy denominator

<!-- focused-spec:id=face-recognition.lfw-fold-mean -->
#### Scenario: Every fold contributes equally

- **WHEN** the ten folds are scored
- **THEN** the reported accuracy is the arithmetic mean of the ten fold accuracies, not the pooled share of all 6,000 pairs

### Requirement: Every calibration candidate is scored on the same pairs

The calibration SHALL score every candidate configuration it compares — the
currently shipped threshold, the threshold sweep and any embedding or model
variant — against the identical LFW pair set and fold split, and SHALL report the
scores together.

<!-- focused-spec:id=face-recognition.lfw-ab-table -->
#### Scenario: A/B table compares candidates on identical pairs

- **WHEN** the calibration finishes with more than one candidate configuration
- **THEN** it prints one row per candidate with the same pair count and fold count, and the accuracy of each candidate

<!-- focused-spec:id=face-recognition.lfw-threshold-selected -->
#### Scenario: The best-scoring candidate is selected

- **WHEN** the threshold sweep finds a value with higher accuracy than the currently shipped threshold
- **THEN** the calibration reports that value as the selected threshold and reports the accuracy it achieved

### Requirement: The shipped recognition threshold is the calibrated one

The recognition similarity floor consumed by the core recognition job SHALL equal
the threshold selected by the LFW calibration and SHALL be the value reported by
`GET /faces/health` as the recognition threshold.

<!-- focused-spec:id=face-recognition.lfw-threshold-reported -->
#### Scenario: Health reports the calibrated recognition threshold

- **WHEN** the calibration has selected a recognition threshold and the faces service starts
- **THEN** `GET /faces/health` reports that threshold and the core recognition job uses it

<!-- focused-spec:id=face-recognition.lfw-floors-ordered -->
#### Scenario: No clustering floor sits below the recognition threshold

- **WHEN** the calibrated recognition threshold is adopted
- **THEN** every clustering similarity floor remains greater than or equal to it

<!-- focused-spec:id=face-recognition.lfw-threshold-unavailable -->
#### Scenario: A service without the threshold is never guessed

- **WHEN** the faces service is unreachable or does not report a recognition threshold
- **THEN** the recognition job substitutes no value of its own: it queries no candidates at a guessed radius, skips merges that would use a guessed floor, and clusters in memory or retries the pass

### Requirement: The LFW dataset is fetched on demand and removed after success

The calibration SHALL download LFW into a temporary work directory at run time,
require no dataset in the repository or on the host, and delete the archive,
images and derived embeddings after a successful run.

<!-- focused-spec:id=face-recognition.lfw-downloaded-on-demand -->
#### Scenario: A run without a local copy downloads LFW

- **WHEN** calibration starts and no local LFW copy exists
- **THEN** it downloads the archive and pairs file, and validates the pairs file contains 6,000 pairs across 10 folds before scoring

<!-- focused-spec:id=face-recognition.lfw-dataset-removed -->
#### Scenario: A successful run leaves no dataset behind

- **WHEN** a calibration run completes successfully
- **THEN** the temporary work directory no longer contains LFW images, the archive or derived embeddings

<!-- focused-spec:id=face-recognition.lfw-dataset-retained-on-failure -->
#### Scenario: A failed run keeps the workspace for diagnosis

- **WHEN** a calibration run fails
- **THEN** the work directory is retained and its path is printed, unless cleanup is explicitly requested

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
