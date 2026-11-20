package database

import (
	"fmt"
	"log"
	"sort"

	"github.com/flashbacks/core/internal/domain"

	"gorm.io/gorm"
)

// maxPersonExemplars mirrors the catalog's exemplar limit: a person keeps at
// most five reference vectors, enough to hold the modes of one identity.
const maxPersonExemplars = 5

// nameKeyIndexSQL keeps person names unique at the storage layer. The index is
// partial, so the unnamed people — whose key is empty — stay exempt.
const nameKeyIndexSQL = `CREATE UNIQUE INDEX IF NOT EXISTS idx_people_name_key_unique ` +
	`ON people (name_key) WHERE name_key <> ''`

// migratePersonNames makes people.name_key authoritative before the unique index
// is created: it backfills the keys, folds the people whose names are equal after
// normalization into the lowest id, and then creates the partial unique index.
// Every step is idempotent, so it runs harmlessly on each start.
func migratePersonNames(db *gorm.DB) {
	if err := backfillPersonNameKeys(db); err != nil {
		log.Printf("warning: backfill people.name_key: %v", err)
	}
	if err := mergeDuplicatePersonNames(db); err != nil {
		log.Printf("warning: merge duplicate person names: %v", err)
	}
	if err := db.Exec(nameKeyIndexSQL).Error; err != nil {
		// Same policy as idx_users_login_lower: a duplicate the repair could not
		// fold must not stop the service from booting. The catalog check keeps
		// enforcing the rule for every write through the API.
		log.Printf("warning: failed to create unique index idx_people_name_key_unique "+
			"(resolve duplicate person names first): %v", err)
	}
}

// backfillPersonNameKeys recomputes every named person's key in Go, so the key
// never depends on the database collation (PostgreSQL lower() and SQLite differ,
// and a C collation leaves Cyrillic untouched).
func backfillPersonNameKeys(db *gorm.DB) error {
	type row struct {
		ID      uint
		Name    string
		NameKey string
	}

	var rows []row
	if err := db.Model(&domain.Person{}).
		Select("id, name, name_key").
		Where("name <> ''").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("list named people: %w", err)
	}

	for _, entry := range rows {
		key := domain.PersonNameKey(entry.Name)
		if key == entry.NameKey {
			continue
		}
		if err := db.Model(&domain.Person{}).Where("id = ?", entry.ID).
			Update("name_key", key).Error; err != nil {
			return fmt.Errorf("backfill person %d: %w", entry.ID, err)
		}
	}
	return nil
}

// mergeDuplicatePersonNames folds every group of people whose names are equal
// after normalization into the person with the lowest id. A group that cannot be
// folded is logged and skipped, never fatal: one broken name must not keep the
// service down.
func mergeDuplicatePersonNames(db *gorm.DB) error {
	var people []domain.Person
	if err := db.Order("id").Find(&people).Error; err != nil {
		return fmt.Errorf("list people: %w", err)
	}

	groups := make(map[string][]domain.Person, len(people))
	for _, person := range people {
		key := domain.PersonNameKey(person.Name)
		if key == "" {
			// Unnamed people never collide: the partial index exempts them.
			continue
		}
		groups[key] = append(groups[key], person)
	}

	keys := make([]string, 0, len(groups))
	for key, group := range groups {
		if len(group) > 1 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		group := groups[key]
		survivor := group[0]
		losers := group[1:]

		if err := mergePersonGroup(db, survivor, losers); err != nil {
			log.Printf("warning: merge people sharing name key %q into person %d: %v",
				key, survivor.ID, err)
			continue
		}
		log.Printf("merged %d duplicate people into person %d (name key %q)",
			len(losers), survivor.ID, key)
	}
	return nil
}

// mergePersonGroup folds the losers into the survivor in one transaction: their
// faces reference the survivor and are locked as user decisions, the cover is
// adopted when the survivor has none, and the exemplars are unioned.
func mergePersonGroup(db *gorm.DB, survivor domain.Person, losers []domain.Person) error {
	loserIDs := make([]uint, 0, len(losers))
	for _, loser := range losers {
		loserIDs = append(loserIDs, loser.ID)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		// Every face involved is a user decision from now on, exactly like a
		// catalog merge: the job must never re-cluster a repaired person.
		if err := tx.Model(&domain.Face{}).
			Where("person_id IN ?", append([]uint{survivor.ID}, loserIDs...)).
			Update("manual_assignment", true).Error; err != nil {
			return fmt.Errorf("lock faces of person %d: %w", survivor.ID, err)
		}
		if err := tx.Model(&domain.Face{}).Where("person_id IN ?", loserIDs).
			Update("person_id", survivor.ID).Error; err != nil {
			return fmt.Errorf("move faces to person %d: %w", survivor.ID, err)
		}

		updates := map[string]any{
			"name_key": domain.PersonNameKey(survivor.Name),
			// The repair is a user-level decision: the recognition job must not
			// re-cluster a person it merged on its own.
			"manual": true,
		}
		if survivor.CoverFaceID == nil {
			for _, loser := range losers {
				if loser.CoverFaceID != nil {
					updates["cover_face_id"] = *loser.CoverFaceID
					break
				}
			}
		}
		if err := tx.Model(&domain.Person{}).Where("id = ?", survivor.ID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("update survivor %d: %w", survivor.ID, err)
		}

		if err := foldReferences(tx, survivor.ID, loserIDs); err != nil {
			return err
		}
		return tx.Where("id IN ?", loserIDs).Delete(&domain.Person{}).Error
	})
}

// foldReferences replaces the exemplars of the merged people with the union of
// their sets: unique by face, capped at maxPersonExemplars, the survivor's own
// vectors first.
func foldReferences(tx *gorm.DB, survivorID uint, loserIDs []uint) error {
	personIDs := append([]uint{survivorID}, loserIDs...)

	var stored []domain.PersonReference
	if err := tx.Where("person_id IN ?", personIDs).
		Order("person_id, quality DESC, id").
		Find(&stored).Error; err != nil {
		return fmt.Errorf("list references of people %v: %w", personIDs, err)
	}

	kept := make([]domain.PersonReference, 0, maxPersonExemplars)
	seen := make(map[uint]bool, maxPersonExemplars)
	for _, reference := range stored {
		if seen[reference.FaceID] {
			continue
		}
		seen[reference.FaceID] = true
		reference.ID = 0
		reference.PersonID = survivorID
		kept = append(kept, reference)
		if len(kept) == maxPersonExemplars {
			break
		}
	}

	if err := tx.Where("person_id IN ?", personIDs).Delete(&domain.PersonReference{}).Error; err != nil {
		return fmt.Errorf("delete references of people %v: %w", personIDs, err)
	}
	if len(kept) == 0 {
		return nil
	}
	return tx.Create(&kept).Error
}
