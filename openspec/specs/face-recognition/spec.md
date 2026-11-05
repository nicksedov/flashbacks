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
