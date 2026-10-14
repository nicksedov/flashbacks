# duplicate-detection Specification

## Purpose

Defines how the core service surfaces groups of files that share identical content, how it reports the empty case, and how resolving a duplicate group removes it, so the base "find and resolve duplicates" journey is specified and regression-tested.

## Requirements

### Requirement: Duplicate grouping by identical content

`GET /api/duplicates` SHALL return groups of files that share identical content. Each group SHALL list every file in the group together with the group size, and the response SHALL report `totalGroups` and `totalFiles`.

<!-- focused-spec:id=duplicate-detection.groups-identical-content -->
#### Scenario: Identical files are grouped

- **WHEN** two indexed files have byte-identical content and a scan has completed
- **THEN** `GET /api/duplicates` returns one group whose `files` array contains the paths of both files

### Requirement: Empty duplicates result

`GET /api/duplicates` SHALL return an empty group list and zero totals when no two indexed files share identical content.

<!-- focused-spec:id=duplicate-detection.no-duplicates-empty -->
#### Scenario: No duplicates yields empty result

- **WHEN** no two indexed files share identical content
- **THEN** `GET /api/duplicates` returns `groups: []`, `totalGroups: 0`, and `totalFiles: 0`

### Requirement: Duplicate resolution removes the group

Deleting one member of a two-member duplicate group by moving it to the trash SHALL cause the group to disappear from the duplicates listing once fewer than two copies remain.

<!-- focused-spec:id=duplicate-detection.resolve-removes-group -->
#### Scenario: Resolving a duplicate clears it

- **WHEN** one file of a two-member duplicate group is deleted through `POST /api/delete-files`
- **THEN** `GET /api/duplicates` no longer returns that group
