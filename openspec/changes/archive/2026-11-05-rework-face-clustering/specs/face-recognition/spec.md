# Spec Delta — face-recognition

## MODIFIED Requirements

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
