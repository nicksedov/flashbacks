-- =============================================================================
-- Production migration: detach faces below the size floor
-- =============================================================================
-- Criteria (see openspec/specs/face-recognition/spec.md and docs/faces/verification-calibration.md):
--   a face is measured by its SMALLER box side, in stored pixels;
--   below 32 px it can never be clustered, so any link it carries is noise.
--
-- Policy confirmed with the user:
--   * manual_assignment = true       -> never touched (explicit user decision)
--   * people.manual  = true          -> never deleted (name / user-created person)
--   * detached faces stay in the gallery as unassigned detections
--
-- Expected effect on the 2026-11-06 production snapshot (dry run verified):
--   faces assigned   44 773 -> 43 026  (1 747 detached)
--   people            1 727 ->  1 703  (24 emptied, none of them manual)
--   people touched    460
--   manual assignments below the floor: 413, all left in place
--   (2 160 faces are below 32 px in total; 413 of them are manual assignments,
--   which the policy keeps, so the automatic detach is 1 747)
--
-- Run on the production host (psql), step by step. The MCP connection used by
-- the IDE is read-only, so this file is executed by an operator.
-- =============================================================================

-- --- 0. Backup (required) ----------------------------------------------------
--   pg_dump -Fc -t people -t faces -t person_references -t face_embeddings \
--     flashbacks > /var/backups/flashbacks-before-small-face-detach.dump

-- --- 1. Dry run: what will change (read-only, run first) ---------------------
SELECT
  count(*) FILTER (
    WHERE person_id > 0 AND manual_assignment = false
      AND (CASE WHEN width < height THEN width ELSE height END) < 32
  ) AS faces_to_detach,
  count(DISTINCT person_id) FILTER (
    WHERE person_id > 0 AND manual_assignment = false
      AND (CASE WHEN width < height THEN width ELSE height END) < 32
  ) AS people_touched,
  count(*) FILTER (
    WHERE person_id > 0 AND manual_assignment = true
      AND (CASE WHEN width < height THEN width ELSE height END) < 32
  ) AS manual_assignments_kept
FROM faces;

-- People that lose every face (expected: 24, none of them manual).
WITH kept AS (
  SELECT f.person_id
  FROM faces f
  WHERE f.person_id > 0
    AND (f.manual_assignment = true
         OR (CASE WHEN f.width < f.height THEN f.width ELSE f.height END) >= 32)
)
SELECT count(*) AS people_emptied,
       count(*) FILTER (WHERE p.manual) AS people_emptied_manual
FROM people p
WHERE EXISTS (SELECT 1 FROM faces f WHERE f.person_id = p.id)
  AND p.id NOT IN (SELECT person_id FROM kept);

-- --- 2. Apply (single transaction) ------------------------------------------
BEGIN;

-- 2a. Detach the faces the service will no longer cluster.
UPDATE faces
SET person_id = 0,
    updated_at = now()
WHERE person_id > 0
  AND manual_assignment = false
  AND (CASE WHEN width < height THEN width ELSE height END) < 32;

-- 2b. Drop exemplars that pointed at a face which just left its person.
DELETE FROM person_references pr
WHERE NOT EXISTS (
  SELECT 1
  FROM faces f
  WHERE f.id = pr.face_id
    AND f.person_id = pr.person_id
);

-- 2c. Recompute the covers of the surviving people, mirroring chooseCover:
--     a full-size face first, then a solo photo, then the most confident.
UPDATE people p
SET cover_face_id = sub.face_id,
    updated_at = now()
FROM (
  SELECT DISTINCT ON (f.person_id) f.person_id, f.id AS face_id
  FROM faces f
  WHERE f.person_id > 0
  ORDER BY f.person_id,
           (CASE WHEN (CASE WHEN f.width < f.height THEN f.width ELSE f.height END) >= 64
                 THEN 0 ELSE 1 END),
           (CASE WHEN (SELECT count(*) FROM faces x
                       WHERE x.image_file_id = f.image_file_id) = 1
                 THEN 0 ELSE 1 END),
           f.confidence DESC,
           f.id
) AS sub
WHERE p.id = sub.person_id
  AND (p.cover_face_id IS NULL OR p.cover_face_id <> sub.face_id);

-- 2d. Remove people nothing points at any more (generated people only).
DELETE FROM person_references
WHERE person_id IN (
  SELECT p.id
  FROM people p
  WHERE p.manual = false
    AND NOT EXISTS (SELECT 1 FROM faces f WHERE f.person_id = p.id)
);

DELETE FROM people p
WHERE p.manual = false
  AND NOT EXISTS (SELECT 1 FROM faces f WHERE f.person_id = p.id);

-- 2e. Verification inside the transaction, before COMMIT.
SELECT count(*) FILTER (WHERE person_id > 0) AS faces_assigned,
       count(*) FILTER (WHERE person_id = 0) AS faces_unassigned,
       (SELECT count(*) FROM people) AS people_total,
       (SELECT count(*) FROM person_references) AS references_total,
       (SELECT count(*) FROM people p
        WHERE NOT EXISTS (SELECT 1 FROM faces f WHERE f.person_id = p.id)) AS people_without_faces
FROM faces;

-- Inspect the numbers, then finish with COMMIT; use ROLLBACK to abandon.
COMMIT;
