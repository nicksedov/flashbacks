# Spec Delta — face-clustering

## ADDED Requirements

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
