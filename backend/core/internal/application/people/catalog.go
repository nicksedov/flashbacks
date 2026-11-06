// Package people owns the user-facing curation of the face catalog: renaming,
// merging, disbanding, creating people from faces and assigning or detaching a
// single face.
//
// Every mutation updates the rows a later clustering pass reads — the manual
// flags on people and faces — so the background job never undoes a user's
// decision, and keeps the person-reference exemplars consistent with the
// resulting people.
package people

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"

	"gorm.io/gorm"
)

// Sentinel errors the HTTP layer maps to status codes.
var (
	// ErrPersonNotFound means the addressed person does not exist.
	ErrPersonNotFound = errors.New("people: person not found")
	// ErrFaceNotFound means the addressed face does not exist (or, on create,
	// already belongs to a person).
	ErrFaceNotFound = errors.New("people: face not found")
	// ErrInvalidInput means the request payload is not acceptable; nothing was
	// changed.
	ErrInvalidInput = errors.New("people: invalid input")
)

const (
	// maxNameLength is the documented limit of a person's display name.
	maxNameLength = 200
	// exemplarLimit mirrors the recognition job: one person keeps at most five
	// reference vectors, enough to hold the modes of one identity.
	exemplarLimit = 5
	// maxMergeSources caps one merge so a single transaction stays bounded.
	maxMergeSources = 50
	// maxCreateFaces caps how many faces one creation request may carry.
	maxCreateFaces = 200
	// defaultPageSize and maxPageSize mirror the flexible pagination mode the
	// rest of the API uses.
	defaultPageSize = 50
	maxPageSize     = 200
)

// UnassignedFace is one face that belongs to no person, together with the photo
// it was detected on. Path is the gallery path the thumbnail endpoint serves.
type UnassignedFace struct {
	Face     domain.Face
	Path     string
	FileName string
}

// UnassignedPage is one page of the unassigned-faces listing.
type UnassignedPage struct {
	Faces       []UnassignedFace
	TotalFaces  int
	CurrentPage int
	PageSize    int
	TotalPages  int
	HasNextPage bool
}

// Catalog is the transactional application service behind the catalog
// mutations. The HTTP layer parses input and builds responses; this type owns
// the business rules and the transaction boundaries.
type Catalog struct {
	db         *gorm.DB
	persons    repository.PersonRepository
	faces      repository.FaceRepository
	references repository.PersonReferenceRepository
	// images is the photo lookup the HTTP layer shares with the listing; the
	// catalog itself joins image_files directly to keep the pagination count
	// consistent with the returned page.
	images repository.ImageFileRepository
}

// NewCatalog creates the catalog service.
func NewCatalog(
	db *gorm.DB,
	persons repository.PersonRepository,
	faces repository.FaceRepository,
	references repository.PersonReferenceRepository,
	images repository.ImageFileRepository,
) *Catalog {
	return &Catalog{
		db:         db,
		persons:    persons,
		faces:      faces,
		references: references,
		images:     images,
	}
}

// Rename sets (or clears) a person's display name and marks the person as
// curated, so a later clustering pass leaves it alone. A blank name clears it.
func (c *Catalog) Rename(personID uint, name string) (*domain.Person, error) {
	trimmed, err := normalizeName(name)
	if err != nil {
		return nil, err
	}

	if _, err := c.loadPerson(personID); err != nil {
		return nil, err
	}

	if err := c.db.Transaction(func(tx *gorm.DB) error {
		return tx.Model(&domain.Person{}).Where("id = ?", personID).Updates(map[string]any{
			"name":   trimmed,
			"manual": true,
		}).Error
	}); err != nil {
		return nil, fmt.Errorf("rename person %d: %w", personID, err)
	}

	return c.loadPerson(personID)
}

// Merge folds every source person into the target in one transaction: their
// faces reference the target and are locked as user-assigned, the sources are
// deleted, and the target's exemplars are the union of every set (capped at
// five) so a later pass still recognises the merged-away identities.
func (c *Catalog) Merge(targetPersonID uint, sourcePersonIDs []uint, name *string) (*domain.Person, error) {
	sources, err := normalizeSources(targetPersonID, sourcePersonIDs)
	if err != nil {
		return nil, err
	}

	var nameToStore *string
	if name != nil {
		trimmed, err := normalizeName(*name)
		if err != nil {
			return nil, err
		}
		nameToStore = &trimmed
	}

	target, err := c.loadPerson(targetPersonID)
	if err != nil {
		return nil, err
	}

	sourcePeople := make([]domain.Person, 0, len(sources))
	for _, sourceID := range sources {
		source, err := c.loadPerson(sourceID)
		if err != nil {
			return nil, err
		}
		sourcePeople = append(sourcePeople, *source)
	}

	references, err := c.unionReferences(targetPersonID, sources)
	if err != nil {
		return nil, err
	}

	cover := target.CoverFaceID
	if cover == nil {
		for _, source := range sourcePeople {
			if source.CoverFaceID != nil {
				cover = source.CoverFaceID
				break
			}
		}
	}

	if err := c.db.Transaction(func(tx *gorm.DB) error {
		// Every face involved is a user decision from now on, so the job never
		// re-clusters a merged face.
		if err := tx.Model(&domain.Face{}).
			Where("person_id IN ?", append([]uint{targetPersonID}, sources...)).
			Update("manual_assignment", true).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Face{}).
			Where("person_id IN ?", sources).
			Update("person_id", targetPersonID).Error; err != nil {
			return err
		}

		updates := map[string]any{
			"manual":        true,
			"cover_face_id": cover,
		}
		if nameToStore != nil {
			updates["name"] = *nameToStore
		}
		if err := tx.Model(&domain.Person{}).Where("id = ?", targetPersonID).
			Updates(updates).Error; err != nil {
			return err
		}

		if err := replaceReferencesTx(tx, targetPersonID, references); err != nil {
			return err
		}
		if err := deleteReferencesTx(tx, sources); err != nil {
			return err
		}
		return tx.Where("id IN ?", sources).Delete(&domain.Person{}).Error
	}); err != nil {
		return nil, fmt.Errorf("merge people: %w", err)
	}

	return c.loadPerson(targetPersonID)
}

// CreateFromFaces creates a manual person from previously unassigned faces.
// Every face is assigned and locked, and the new person's exemplars are seeded
// from the faces' embeddings so a later pass recognises the identity.
func (c *Catalog) CreateFromFaces(name string, faceIDs []uint) (*domain.Person, error) {
	ids, err := normalizeFaceIDs(faceIDs)
	if err != nil {
		return nil, err
	}

	trimmed, err := normalizeName(name)
	if err != nil {
		return nil, err
	}

	faces := make([]domain.Face, 0, len(ids))
	for _, faceID := range ids {
		face, err := c.loadFace(faceID)
		if err != nil {
			return nil, err
		}
		if face.PersonID != 0 {
			// The endpoint answers 404 for an unknown or already-assigned face,
			// so the caller cannot silently steal a face from a person.
			return nil, ErrFaceNotFound
		}
		faces = append(faces, *face)
	}

	embeddings, err := c.loadEmbeddings(ids)
	if err != nil {
		return nil, err
	}
	candidates := chooseExemplars(faces, embeddings)

	cover := faces[0].ID
	if len(candidates) > 0 {
		cover = candidates[0].faceID
	}

	person := domain.Person{
		Name:        trimmed,
		Manual:      true,
		CoverFaceID: &cover,
	}

	references := make([]domain.PersonReference, 0, len(candidates))
	for _, candidate := range candidates {
		references = append(references, domain.PersonReference{
			FaceID:    candidate.faceID,
			Dimension: len(candidate.vector),
			Embedding: domain.FormatEmbedding(candidate.vector),
			Quality:   candidate.quality,
		})
	}

	if err := c.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&person).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Face{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"person_id":         person.ID,
				"manual_assignment": true,
			}).Error; err != nil {
			return err
		}
		return replaceReferencesTx(tx, person.ID, references)
	}); err != nil {
		return nil, fmt.Errorf("create person from faces: %w", err)
	}

	return c.loadPerson(person.ID)
}

// Disband makes every face of a person unassigned and locked, drops the
// person's exemplars and removes the person. Photos and face rows survive.
func (c *Catalog) Disband(personID uint) error {
	person, err := c.loadPerson(personID)
	if err != nil {
		return err
	}

	if err := c.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.Face{}).Where("person_id = ?", personID).
			Updates(map[string]any{
				"person_id":         0,
				"manual_assignment": true,
			}).Error; err != nil {
			return err
		}
		if err := deleteReferencesTx(tx, []uint{personID}); err != nil {
			return err
		}
		return tx.Where("id = ?", personID).Delete(&domain.Person{}).Error
	}); err != nil {
		return fmt.Errorf("disband person %d: %w", personID, err)
	}

	_ = person
	return nil
}

// AssignFace moves a face to a person, or detaches it when personID is 0. The
// face is locked so a later pass leaves it alone; the exemplars of the previous
// and the new person follow the edit, and a cover face leaving its person
// promotes the person's lowest-id remaining face.
func (c *Catalog) AssignFace(faceID, personID uint) (*domain.Face, error) {
	face, err := c.loadFace(faceID)
	if err != nil {
		return nil, err
	}

	if personID != 0 {
		if _, err := c.loadPerson(personID); err != nil {
			return nil, err
		}
	}

	previous := face.PersonID
	if previous == personID {
		// A no-op assignment still locks the face: the user decided its person.
		if err := c.db.Model(&domain.Face{}).Where("id = ?", faceID).
			Update("manual_assignment", true).Error; err != nil {
			return nil, fmt.Errorf("lock face %d: %w", faceID, err)
		}
		return c.loadFace(faceID)
	}

	var previousPerson *domain.Person
	var previousReferences []domain.PersonReference
	if previous != 0 {
		previousPerson, err = c.loadPerson(previous)
		if err != nil {
			return nil, err
		}
		previousReferences, err = c.references.List(previous)
		if err != nil {
			return nil, fmt.Errorf("list references of person %d: %w", previous, err)
		}
	}

	var newReferences []domain.PersonReference
	if personID != 0 {
		newReferences, err = c.references.List(personID)
		if err != nil {
			return nil, fmt.Errorf("list references of person %d: %w", personID, err)
		}
		embeddings, err := c.loadEmbeddings([]uint{faceID})
		if err != nil {
			return nil, err
		}
		newReferences = appendFaceExemplar(newReferences, *face, embeddings[faceID])
	}

	if err := c.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.Face{}).Where("id = ?", faceID).
			Updates(map[string]any{
				"person_id":         personID,
				"manual_assignment": true,
			}).Error; err != nil {
			return err
		}

		if previous != 0 {
			remaining := dropFaceReference(previousReferences, faceID)
			if err := replaceReferencesTx(tx, previous, remaining); err != nil {
				return err
			}
			if previousPerson.CoverFaceID != nil && *previousPerson.CoverFaceID == faceID {
				if err := promoteCover(tx, previous); err != nil {
					return err
				}
			}
		}

		if personID != 0 {
			return replaceReferencesTx(tx, personID, newReferences)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("assign face %d to person %d: %w", faceID, personID, err)
	}

	return c.loadFace(faceID)
}

// ListUnassigned returns a page of faces that belong to no person, each with
// its photo path. Faces whose photo no longer exists are skipped, and a face a
// user detached is listed: it is unassigned unless the user assigns it again.
func (c *Catalog) ListUnassigned(page, pageSize int) (*UnassignedPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}

	base := c.db.Table("faces").
		Joins("INNER JOIN image_files ON image_files.id = faces.image_file_id").
		Where("faces.person_id = ?", 0)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count unassigned faces: %w", err)
	}

	totalPages := int(total)
	if totalPages > 0 {
		totalPages = (totalPages + pageSize - 1) / pageSize
	}
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	type row struct {
		ID          uint
		ImageFileID uint
		PersonID    uint
		X           float32
		Y           float32
		Width       float32
		Height      float32
		Confidence  float32
		Path        string
	}
	var rows []row
	if err := base.Select(
		"faces.id, faces.image_file_id, faces.person_id, faces.x, faces.y, " +
			"faces.width, faces.height, faces.confidence, image_files.path",
	).Order("faces.id").Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list unassigned faces: %w", err)
	}

	faces := make([]UnassignedFace, 0, len(rows))
	for _, result := range rows {
		faces = append(faces, UnassignedFace{
			Face: domain.Face{
				ID:          result.ID,
				ImageFileID: result.ImageFileID,
				PersonID:    result.PersonID,
				X:           result.X,
				Y:           result.Y,
				Width:       result.Width,
				Height:      result.Height,
				Confidence:  result.Confidence,
			},
			Path:     result.Path,
			FileName: filepath.Base(result.Path),
		})
	}

	return &UnassignedPage{
		Faces:       faces,
		TotalFaces:  int(total),
		CurrentPage: page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
		HasNextPage: page < totalPages,
	}, nil
}

// loadPerson returns a person or ErrPersonNotFound.
func (c *Catalog) loadPerson(personID uint) (*domain.Person, error) {
	person, err := c.persons.FindByID(personID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPersonNotFound
		}
		return nil, fmt.Errorf("load person %d: %w", personID, err)
	}
	return person, nil
}

// loadFace returns a face or ErrFaceNotFound.
func (c *Catalog) loadFace(faceID uint) (*domain.Face, error) {
	face, err := c.faces.FindByID(faceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFaceNotFound
		}
		return nil, fmt.Errorf("load face %d: %w", faceID, err)
	}
	return face, nil
}

// loadEmbeddings reads the stored vectors of the given faces.
func (c *Catalog) loadEmbeddings(faceIDs []uint) (map[uint][]float32, error) {
	vectors := make(map[uint][]float32, len(faceIDs))
	if len(faceIDs) == 0 {
		return vectors, nil
	}

	var rows []domain.FaceEmbedding
	if err := c.db.Where("face_id IN ?", faceIDs).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load face embeddings: %w", err)
	}
	for _, row := range rows {
		vector, err := domain.ParseEmbedding(row.Embedding)
		if err != nil || len(vector) == 0 {
			continue
		}
		vectors[row.FaceID] = vector
	}
	return vectors, nil
}

// unionReferences combines the exemplars of the target and the sources, target
// first, without duplicating a face and capped at exemplarLimit.
func (c *Catalog) unionReferences(targetID uint, sourceIDs []uint) ([]domain.PersonReference, error) {
	sets := make([][]domain.PersonReference, 0, len(sourceIDs)+1)
	targetReferences, err := c.references.List(targetID)
	if err != nil {
		return nil, fmt.Errorf("list references of person %d: %w", targetID, err)
	}
	sets = append(sets, targetReferences)
	for _, sourceID := range sourceIDs {
		sourceReferences, err := c.references.List(sourceID)
		if err != nil {
			return nil, fmt.Errorf("list references of person %d: %w", sourceID, err)
		}
		sets = append(sets, sourceReferences)
	}
	return mergeReferenceSets(sets), nil
}

// mergeReferenceSets concatenates reference sets, dropping duplicate face ids
// and stored row ids (Replace re-inserts the rows), capped at exemplarLimit.
func mergeReferenceSets(sets [][]domain.PersonReference) []domain.PersonReference {
	merged := make([]domain.PersonReference, 0, exemplarLimit)
	seen := make(map[uint]bool, exemplarLimit)
	for _, set := range sets {
		for _, reference := range set {
			if seen[reference.FaceID] {
				continue
			}
			seen[reference.FaceID] = true
			reference.ID = 0
			merged = append(merged, reference)
			if len(merged) == exemplarLimit {
				return merged
			}
		}
	}
	return merged
}

// dropFaceReference returns the reference set without one face.
func dropFaceReference(references []domain.PersonReference, faceID uint) []domain.PersonReference {
	kept := make([]domain.PersonReference, 0, len(references))
	for _, reference := range references {
		if reference.FaceID == faceID {
			continue
		}
		reference.ID = 0
		kept = append(kept, reference)
	}
	return kept
}

// appendFaceExemplar adds a face's vector to a person's reference set, keeping
// the set bounded and free of duplicate faces.
func appendFaceExemplar(
	references []domain.PersonReference, face domain.Face, vector []float32,
) []domain.PersonReference {
	kept := dropFaceReference(references, face.ID)
	if len(vector) == 0 {
		return kept
	}
	kept = append(kept, domain.PersonReference{
		FaceID:    face.ID,
		Dimension: len(vector),
		Embedding: domain.FormatEmbedding(vector),
		Quality:   face.Confidence,
	})
	if len(kept) > exemplarLimit {
		kept = kept[:exemplarLimit]
	}
	return kept
}

// promoteCover makes a person's lowest-id remaining face the cover, or clears
// the cover when the person has no face left.
func promoteCover(tx *gorm.DB, personID uint) error {
	var remaining domain.Face
	err := tx.Where("person_id = ?", personID).Order("id").First(&remaining).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Model(&domain.Person{}).Where("id = ?", personID).
			Update("cover_face_id", nil).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&domain.Person{}).Where("id = ?", personID).
		Update("cover_face_id", remaining.ID).Error
}

// replaceReferencesTx stores a person's exemplars inside a transaction.
func replaceReferencesTx(tx *gorm.DB, personID uint, references []domain.PersonReference) error {
	if err := tx.Where("person_id = ?", personID).Delete(&domain.PersonReference{}).Error; err != nil {
		return err
	}
	if len(references) == 0 {
		return nil
	}
	for index := range references {
		references[index].PersonID = personID
		references[index].ID = 0
	}
	return tx.Create(&references).Error
}

// deleteReferencesTx removes the exemplars of several people inside a
// transaction.
func deleteReferencesTx(tx *gorm.DB, personIDs []uint) error {
	if len(personIDs) == 0 {
		return nil
	}
	return tx.Where("person_id IN ?", personIDs).Delete(&domain.PersonReference{}).Error
}

// normalizeName trims a display name and rejects one longer than maxNameLength
// characters. An empty result means "unnamed".
func normalizeName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if utf8.RuneCountInString(trimmed) > maxNameLength {
		return "", ErrInvalidInput
	}
	return trimmed, nil
}

// normalizeSources validates the sources of a merge: at least one, at most
// maxMergeSources distinct ids, and never the target itself.
func normalizeSources(targetID uint, sourceIDs []uint) ([]uint, error) {
	if len(sourceIDs) == 0 {
		return nil, ErrInvalidInput
	}
	seen := make(map[uint]bool, len(sourceIDs))
	sources := make([]uint, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		if sourceID == 0 || sourceID == targetID {
			return nil, ErrInvalidInput
		}
		if seen[sourceID] {
			continue
		}
		seen[sourceID] = true
		sources = append(sources, sourceID)
	}
	if len(sources) > maxMergeSources {
		return nil, ErrInvalidInput
	}
	return sources, nil
}

// normalizeFaceIDs validates the faces of a creation request: at least one, at
// most maxCreateFaces distinct ids.
func normalizeFaceIDs(faceIDs []uint) ([]uint, error) {
	if len(faceIDs) == 0 {
		return nil, ErrInvalidInput
	}
	seen := make(map[uint]bool, len(faceIDs))
	ids := make([]uint, 0, len(faceIDs))
	for _, faceID := range faceIDs {
		if faceID == 0 {
			return nil, ErrInvalidInput
		}
		if seen[faceID] {
			continue
		}
		seen[faceID] = true
		ids = append(ids, faceID)
	}
	if len(ids) > maxCreateFaces {
		return nil, ErrInvalidInput
	}
	return ids, nil
}

// exemplarCandidate is one face chosen as a reference vector.
type exemplarCandidate struct {
	faceID  uint
	quality float32
	vector  []float32
}

// chooseExemplars picks up to exemplarLimit reference faces: the highest
// confidence face of every distinct photo, highest confidence first — the same
// selection rule the recognition job applies.
func chooseExemplars(faces []domain.Face, embeddings map[uint][]float32) []exemplarCandidate {
	bestPerImage := make(map[uint]domain.Face, len(faces))
	for _, face := range faces {
		if len(embeddings[face.ID]) == 0 {
			continue
		}
		current, ok := bestPerImage[face.ImageFileID]
		if !ok || face.Confidence > current.Confidence {
			bestPerImage[face.ImageFileID] = face
		}
	}

	candidates := make([]exemplarCandidate, 0, len(bestPerImage))
	for _, face := range bestPerImage {
		candidates = append(candidates, exemplarCandidate{
			faceID:  face.ID,
			quality: face.Confidence,
			vector:  embeddings[face.ID],
		})
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		if candidates[left].quality != candidates[right].quality {
			return candidates[left].quality > candidates[right].quality
		}
		return candidates[left].faceID < candidates[right].faceID
	})
	if len(candidates) > exemplarLimit {
		candidates = candidates[:exemplarLimit]
	}
	return candidates
}
