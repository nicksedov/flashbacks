# face-clustering Specification

## Purpose

Lets the recognition job group faces into people from a bounded neighbourhood
served by the vector store, so clustering keeps working when the gallery no
longer fits into one in-memory all-pairs matrix, while the calibrated similarity
thresholds stay with the service that owns them.

## Requirements

### Requirement: Clustering links are decided on candidate neighbours

When the embedding store can answer nearest-neighbour queries, the recognition
job SHALL request each unassigned face's neighbours inside the clustering
similarity radius and SHALL decide person links from those candidates instead of
an all-pairs similarity matrix.

<!-- focused-spec:id=face-clustering.candidates-scope-the-graph -->
#### Scenario: Only neighbours inside the radius become candidates

- **WHEN** a face's nearest stored faces sit at similarity 0.9 and 0.2 with a clustering radius of 0.40
- **THEN** only the face at 0.9 is offered to the clustering decision as its candidate

<!-- focused-spec:id=face-clustering.same-person-linked -->
#### Scenario: Two photos of one person share a person through their candidates

- **WHEN** two unassigned faces of the same person are offered as each other's candidate
- **THEN** both faces reference the same person after the job assigns people

### Requirement: The faces service keeps the similarity thresholds

Applying the clustering similarity floor, the relaxed recognition floor and the
runner-up margin SHALL remain the decision of the faces service, which receives
candidate links and returns the resulting person labels.

<!-- focused-spec:id=face-clustering.recognition-threshold-applied -->
#### Scenario: A leftover face joins the group its best candidate names

- **WHEN** a submitted candidate graph gives a leftover face exactly one candidate group at similarity 0.30
- **THEN** the service labels that face with the candidate group's label instead of leaving it unassigned

<!-- focused-spec:id=face-clustering.ambiguous-candidate-unassigned -->
#### Scenario: An ambiguous best candidate stays unassigned

- **WHEN** a leftover face's best two candidate groups score 0.30 and 0.285 with a required margin of 0.05
- **THEN** the service leaves that face unassigned

### Requirement: Clustering still works without neighbour queries

When the store cannot answer nearest-neighbour queries, the recognition job SHALL
cluster the unassigned embeddings in memory so people are still produced.

<!-- focused-spec:id=face-clustering.in-memory-fallback -->
#### Scenario: A store without vector support still yields people

- **WHEN** the job clusters faces on a store that does not support nearest-neighbour queries
- **THEN** the same person's faces still end up referencing one person

### Requirement: The neighbour index is maintained outside the insert path

The embedding store SHALL provide a cosine-distance nearest-neighbour index over
face embeddings, and the job SHALL create it after an analysis pass rather than
on every face insert.

<!-- focused-spec:id=face-clustering.index-after-pass -->
#### Scenario: A pass requests the index once it has stored the new faces

- **WHEN** an analysis pass finishes storing the analysed faces
- **THEN** the job asks the store to ensure the neighbour index exactly once and does so before assigning people

### Requirement: Manual catalog edits are authoritative across clustering passes

A person the user created, merged or renamed, and the person of a face the user
assigned, detached or disbanded, SHALL keep that state through every later
clustering pass. A pass SHALL NOT reassign a user-assigned face and SHALL NOT
re-cluster a face the user detached.

<!-- focused-spec:id=face-clustering.manual-person-survives -->
#### Scenario: A curated person is not dissolved by a pass

- **WHEN** the user merges three people into one and the recognition job runs another clustering pass
- **THEN** the merged person still holds every face of all three and the source people are not recreated

<!-- focused-spec:id=face-clustering.manual-face-survives -->
#### Scenario: A user-assigned face keeps its person

- **WHEN** the user assigns a face to a person and a later pass finds that face closer to another person
- **THEN** the face still references the person the user chose

<!-- focused-spec:id=face-clustering.detached-face-stays-unassigned -->
#### Scenario: A detached face is not clustered again

- **WHEN** the user detaches a face from its person and the recognition job runs another clustering pass
- **THEN** the face still belongs to no person

<!-- focused-spec:id=face-clustering.disbanded-faces-stay-unassigned -->
#### Scenario: Disbanded faces stay unassigned

- **WHEN** the user disbands a person and the recognition job runs another clustering pass
- **THEN** the disbanded faces belong to no person and the person is not recreated

<!-- focused-spec:id=face-clustering.renamed-person-keeps-name -->
#### Scenario: A user-typed name survives a pass

- **WHEN** the user names a person and the recognition job reconciles people in a later pass
- **THEN** the person keeps that name

### Requirement: A merged person keeps the reference embeddings of its sources

Merging people SHALL keep the reference embeddings of every source on the
target, so a later pass recognises a source identity as the merged person
instead of creating a person for it again.

<!-- focused-spec:id=face-clustering.merged-person-recognised-later -->
#### Scenario: A later photo of a source identity joins the merged person

- **WHEN** a person was merged into another person and a later analysis pass examines a new photo of the merged-away identity
- **THEN** that face references the merged person and no new person is created for it

### Requirement: A person created from unassigned faces is recognised later

A person the user created from unassigned faces SHALL keep reference embeddings
of those faces, so a later pass recognises the same identity as that person
instead of creating a person for it again.

<!-- focused-spec:id=face-clustering.created-person-recognised-later -->
#### Scenario: A later photo of a created person joins that person

- **WHEN** the user creates a person from unassigned faces and a later analysis pass examines a new photo of the same identity
- **THEN** that face references the created person and no new person is created for it
