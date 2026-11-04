# face-recognition Specification

## Purpose

Lets users browse the photo library by the people in it: a background job
detects faces on gallery photos, clusters them into unnamed people, and a
People view lists those people and the photos each one appears in.

## Requirements

### Requirement: Faces are grouped into people by the background job

The core service SHALL run a background face recognition job that detects faces
on gallery photos and assigns every detected face to exactly one person
(cluster).

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
