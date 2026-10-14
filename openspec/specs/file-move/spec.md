# file-move Specification

## Purpose

Defines how selected files are moved to a destination directory through the move endpoint, including deterministic handling of name conflicts, so the base "move files" journey is specified and regression-tested.

## Requirements

### Requirement: Move files to a target directory

`POST /api/move-files` SHALL move every path in the request into the target directory and report the number of successful and failed moves.

<!-- focused-spec:id=file-move.move-to-target-dir -->
#### Scenario: Files are moved to the target

- **WHEN** one or more existing files are submitted to `POST /api/move-files` with a writable target directory
- **THEN** the response reports the moves as successful and the files exist in the target directory instead of their original paths

### Requirement: Move resolves name conflicts

When the target directory already contains a file with the same name, `POST /api/move-files` SHALL preserve both files by suffixing the moved file's name rather than overwriting the existing file.

<!-- focused-spec:id=file-move.move-name-conflict -->
#### Scenario: Conflict is suffixed, not overwritten

- **WHEN** a file is moved into a target directory that already contains a different file with the same name
- **THEN** both files exist afterwards and the response reports the move as successful
