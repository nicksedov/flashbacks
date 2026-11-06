package faces

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/flashbacks/core/internal/domain"
)

// reconcileMaxShare/reconcileMaxFacesMin bound how large one merge may grow a
// person during reconcile. A person above this share of the assigned gallery
// would be implausible, so the merge is refused: the guard stops a lookalike
// chain from collapsing the library into one person. It is a safety valve, not
// the splitting mechanism.
//
// 15% leaves room for a large real identity (the production gallery's main
// person holds ~14% of its faces) while still refusing a chain reaction; the
// faces service clusters a single component under its own 10% cap first.
const (
	reconcileMaxShare    = 0.15
	reconcileMaxFacesMin = 250
)

// errThresholdsUnavailable means the faces service did not advertise its
// similarity floors. Core never substitutes its own numbers for them: a pass
// without the service's values degrades (in-memory clustering, no reference
// matches) instead of judging faces against a guessed threshold.
var errThresholdsUnavailable = errors.New("faces service did not report its thresholds")

// clusteringThresholds are the similarity floors and size floors the faces
// service advertises. The service is the single source of truth, so core keeps
// no copies of them.
type clusteringThresholds struct {
	recognition       float64
	assign            float64
	margin            float64
	merge             float64
	mergeSupport      int
	supportSimilarity float64
	// facePx is the smallest clusterable face (smaller box side, px). Zero
	// means the service did not report it, and then no size rule is applied
	// rather than guessed.
	facePx float64
	// weakFacePx is the size from which a face is a full participant. It also
	// marks the smallest face worth storing as an exemplar or a cover.
	weakFacePx float64
}

// referenceVectors groups the exemplar vectors of every person.
type referenceVectors struct {
	vectors      map[uint][][]float32
	faceToPerson map[uint]uint
}

// loadReferenceVectors reads every person's exemplars; nil when the store is
// not configured or holds no reference yet.
func (m *Manager) loadReferenceVectors() *referenceVectors {
	if m.personRefs == nil {
		return nil
	}
	grouped, err := m.personRefs.ListAll()
	if err != nil {
		m.setError(fmt.Sprintf("load person references: %v", err))
		return nil
	}
	result := &referenceVectors{
		vectors:      make(map[uint][][]float32, len(grouped)),
		faceToPerson: make(map[uint]uint),
	}
	for personID, references := range grouped {
		for _, reference := range references {
			vector, err := domain.ParseEmbedding(reference.Embedding)
			if err != nil || len(vector) == 0 {
				continue
			}
			result.vectors[personID] = append(result.vectors[personID], vector)
			result.faceToPerson[reference.FaceID] = personID
		}
	}
	if len(result.vectors) == 0 {
		return nil
	}
	return result
}

// thresholds reads the similarity floors from the faces service.
//
// The recognition floor and the assignment floor are required: a pass that
// invented them would accept or reject faces against numbers nobody
// calibrated, which is exactly what the LFW calibration exists to prevent. A
// service that does not report them yields errThresholdsUnavailable so the
// caller can degrade instead. A missing merge floor is not an error: grouping
// is then simply left to the faces service.
func (m *Manager) thresholds(ctx context.Context) (clusteringThresholds, error) {
	health, err := m.client.Health(ctx)
	if err != nil {
		return clusteringThresholds{}, fmt.Errorf("%w: %v", errThresholdsUnavailable, err)
	}
	resolved := clusteringThresholds{
		recognition:       health.Thresholds.RecognitionMinSimilarity,
		assign:            health.Thresholds.ClusterAssignSimilarity,
		margin:            health.Thresholds.RecognitionMargin,
		merge:             health.Thresholds.ClusterMergeSimilarity,
		mergeSupport:      health.Thresholds.ClusterMergeSupport,
		supportSimilarity: health.Thresholds.ClusterMergeSupportSimilarity,
		facePx:            health.Thresholds.MinClusterableFacePx,
		weakFacePx:        health.Thresholds.WeakFacePx,
	}
	if resolved.recognition <= 0 || resolved.assign <= 0 {
		return clusteringThresholds{}, fmt.Errorf(
			"%w: recognition=%v assign=%v",
			errThresholdsUnavailable, resolved.recognition, resolved.assign,
		)
	}
	return resolved, nil
}

// candidatePersonMap resolves the persons of the candidate faces named by the
// neighbour query, so an unassigned face can only join a person it is actually
// similar to.
func (m *Manager) candidatePersonMap(candidates map[uint][]Neighbour) map[uint]uint {
	if len(candidates) == 0 {
		return nil
	}
	faceIDs := make([]uint, 0, len(candidates))
	seen := make(map[uint]bool, len(candidates))
	for _, neighbours := range candidates {
		for _, neighbour := range neighbours {
			if seen[neighbour.FaceID] {
				continue
			}
			seen[neighbour.FaceID] = true
			faceIDs = append(faceIDs, neighbour.FaceID)
		}
	}

	personByFace := make(map[uint]uint, len(faceIDs))
	for start := 0; start < len(faceIDs); start += embeddingBatch {
		end := min(start+embeddingBatch, len(faceIDs))
		var rows []struct {
			ID       uint
			PersonID uint
		}
		if err := m.db.Model(&domain.Face{}).Select("id, person_id").
			Where("id IN ?", faceIDs[start:end]).Scan(&rows).Error; err != nil {
			m.setError(fmt.Sprintf("load candidate persons: %v", err))
			return personByFace
		}
		for _, row := range rows {
			if row.PersonID > 0 {
				personByFace[row.ID] = row.PersonID
			}
		}
	}
	return personByFace
}

// candidatePersons lists the persons a face may join through its own
// candidates. Without a candidate graph (the in-memory path) every person is a
// candidate, because there is no truncation to gate on.
func (r *referenceVectors) candidatePersons(
	faceID uint, candidates map[uint][]Neighbour, personByFace map[uint]uint,
) map[uint]bool {
	persons := make(map[uint]bool)
	if candidates == nil {
		for personID := range r.vectors {
			persons[personID] = true
		}
		return persons
	}
	for _, neighbour := range candidates[faceID] {
		if personID, ok := personByFace[neighbour.FaceID]; ok {
			persons[personID] = true
		}
	}
	return persons
}

// assignKnownPeople matches unassigned faces against the existing person
// references before anything is sent for clustering. A face joins a person when
// its best reference similarity clears the assignment floor and beats the
// runner-up by the margin; an ambiguous or weak match stays unassigned.
func (m *Manager) assignKnownPeople(
	members []faceRef,
	embeddings [][]float32,
	candidates map[uint][]Neighbour,
	references *referenceVectors,
	personByFace map[uint]uint,
	thresholds clusteringThresholds,
) map[uint]bool {
	if references == nil || len(members) == 0 {
		return nil
	}

	assigned := make(map[uint]bool)
	byPerson := make(map[uint][]uint)
	for index, member := range members {
		if index >= len(embeddings) {
			continue
		}
		persons := references.candidatePersons(member.faceID, candidates, personByFace)
		best, runnerUp, bestPerson := -1.0, -1.0, uint(0)
		for personID := range persons {
			score := bestSimilarity(embeddings[index], references.vectors[personID])
			switch {
			case score > best:
				best, runnerUp, bestPerson = score, best, personID
			case score > runnerUp:
				runnerUp = score
			}
		}
		if bestPerson == 0 || best < thresholds.assign || best-runnerUp < thresholds.margin {
			continue
		}
		assigned[member.faceID] = true
		byPerson[bestPerson] = append(byPerson[bestPerson], member.faceID)
	}

	total := 0
	for personID, faceIDs := range byPerson {
		m.assignFacesToPerson(personID, faceIDs)
		total += len(faceIDs)
	}
	if total > 0 {
		log.Printf("Face recognition: assigned %d faces to existing people", total)
	}
	return assigned
}

// matchClusterToPerson matches a freshly labelled cluster against the existing
// references, so a known identity extends its person instead of minting a new
// one.
func (m *Manager) matchClusterToPerson(
	cluster []faceRef,
	memberVectors map[uint][]float32,
	references *referenceVectors,
	candidates map[uint][]Neighbour,
	personByFace map[uint]uint,
	thresholds clusteringThresholds,
) (uint, bool) {
	if references == nil || len(cluster) == 0 {
		return 0, false
	}

	vectors := make([][]float32, 0, len(cluster))
	for _, member := range cluster {
		if vector := memberVectors[member.faceID]; len(vector) > 0 {
			vectors = append(vectors, vector)
		}
	}
	if len(vectors) == 0 {
		return 0, false
	}
	median := meanVector(vectors)

	persons := make(map[uint]bool)
	for _, member := range cluster {
		for personID := range references.candidatePersons(member.faceID, candidates, personByFace) {
			persons[personID] = true
		}
	}
	best, runnerUp, bestPerson := -1.0, -1.0, uint(0)
	for personID := range persons {
		score := bestSimilarity(median, references.vectors[personID])
		switch {
		case score > best:
			best, runnerUp, bestPerson = score, best, personID
		case score > runnerUp:
			runnerUp = score
		}
	}
	if bestPerson == 0 || best < thresholds.assign || best-runnerUp < thresholds.margin {
		return 0, false
	}
	return bestPerson, true
}

// replaceReferences stores the exemplars of one person from its member faces.
//
// Only full-size faces become exemplars: a weak face drifts far enough from the
// person's own portfolio to pull a later assignment towards a lookalike. A
// cluster that holds nothing but weak faces keeps them, because a person
// without exemplars can never be matched again. *minSize* is the service's
// full-size floor; zero disables it.
func (m *Manager) replaceReferences(
	personID uint,
	cluster []faceRef,
	memberVectors map[uint][]float32,
	facesPerImage map[uint]int,
	minSize float32,
) {
	if m.personRefs == nil {
		return
	}

	if minSize > 0 {
		fullSize := make([]faceRef, 0, len(cluster))
		for _, member := range cluster {
			if minFaceSide(member.width, member.height) >= minSize {
				fullSize = append(fullSize, member)
			}
		}
		if len(fullSize) > 0 {
			cluster = fullSize
		}
	}

	type candidate struct {
		faceID     uint
		confidence float32
		solo       bool
		vector     []float32
	}
	candidates := make([]candidate, 0, len(cluster))
	for _, member := range cluster {
		vector := memberVectors[member.faceID]
		if len(vector) == 0 {
			continue
		}
		candidates = append(candidates, candidate{
			faceID:     member.faceID,
			confidence: member.confidence,
			solo:       facesPerImage[member.imageFileID] <= 1,
			vector:     vector,
		})
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		if candidates[left].solo != candidates[right].solo {
			return candidates[left].solo
		}
		return candidates[left].confidence > candidates[right].confidence
	})
	if len(candidates) > exemplarLimit {
		candidates = candidates[:exemplarLimit]
	}

	references := make([]domain.PersonReference, 0, len(candidates))
	for _, chosen := range candidates {
		references = append(references, domain.PersonReference{
			FaceID:    chosen.faceID,
			Dimension: len(chosen.vector),
			Embedding: domain.FormatEmbedding(chosen.vector),
			Quality:   chosen.confidence,
		})
	}
	if err := m.personRefs.Replace(personID, references); err != nil {
		m.setError(fmt.Sprintf("store references of person %d: %v", personID, err))
	}
}

// rebuildReferences replaces the exemplars of one person from its stored faces.
//
// Only full-size faces become exemplars: a weak face drifts far enough from the
// person's own portfolio to pull a later assignment towards a lookalike. When a
// person holds nothing but weak faces its best faces are kept, because an
// imperfect exemplar still beats a person nobody can match.
func (m *Manager) rebuildReferences(
	personID uint,
	facesPerImage map[uint]int,
	thresholds clusteringThresholds,
) {
	var faces []domain.Face
	if err := m.db.Where("person_id = ?", personID).Order("confidence DESC, id").
		Find(&faces).Error; err != nil {
		m.setError(fmt.Sprintf("load faces of person %d: %v", personID, err))
		return
	}
	if kept := fullSizeFaces(faces, thresholds.weakFacePx); len(kept) > 0 {
		faces = kept
	}
	if len(faces) == 0 {
		if err := m.personRefs.Replace(personID, nil); err != nil {
			m.setError(fmt.Sprintf("clear references of person %d: %v", personID, err))
		}
		return
	}

	faceIDs := make([]uint, len(faces))
	for index, face := range faces {
		faceIDs[index] = face.ID
	}
	vectors := m.loadEmbeddings(faceIDs)

	cluster := make([]faceRef, 0, len(faces))
	memberVectors := make(map[uint][]float32, len(faces))
	for _, face := range faces {
		vector := vectors[face.ID]
		if len(vector) == 0 {
			continue
		}
		cluster = append(cluster, faceRef{
			faceID:      face.ID,
			imageFileID: face.ImageFileID,
			confidence:  face.Confidence,
			width:       face.Width,
			height:      face.Height,
		})
		memberVectors[face.ID] = vector
	}
	m.replaceReferences(
		personID, cluster, memberVectors, facesPerImage, float32(thresholds.weakFacePx),
	)
}

// peopleNeedReconcile reports whether stored people come from another
// clustering generation and therefore owe a reconcile pass.
func (m *Manager) peopleNeedReconcile() bool {
	if m.personRefs == nil {
		return false
	}

	var count int64
	if err := m.db.Model(&domain.Person{}).
		Where("cluster_version <> ?", personClusterVersion).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// reconcilePeople merges the people one identity was spread over.
//
// References are built lazily for people that lack them, medoids are compared,
// and two people merge when their medoid similarity clears the merge floor and
// their exemplar sets share enough cross-support. The oldest id survives, so a
// user's link to a person does not churn more than necessary.
//
// A merge replaces the survivor's reference set, which can unlock another
// merge, so rounds repeat until one changes nothing. A later reconcile pass is
// then a no-op.
func (m *Manager) reconcilePeople() {
	if m.personRefs == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.clusterTimeout)
	defer cancel()
	thresholds, err := m.thresholds(ctx)
	if err != nil {
		// Without the service's floors a merge would compare people against
		// numbers core guessed. Skip the merge rounds; the generation
		// bookkeeping below still runs and the next pass retries.
		log.Printf("Face recognition: reconcile skipped: %v", err)
	} else {
		facesPerImage := m.imageFaceCounts()
		for m.reconcileRound(thresholds, facesPerImage) > 0 {
		}
	}

	// People without references could not be compared; they still belong to
	// the current generation.
	if err := m.db.Model(&domain.Person{}).
		Where("cluster_version <> ?", personClusterVersion).
		Update("cluster_version", personClusterVersion).Error; err != nil {
		m.setError(fmt.Sprintf("mark people reconciled: %v", err))
	}
}

// reconcileRound compares every pair of people once and returns how many
// people it merged away.
func (m *Manager) reconcileRound(
	thresholds clusteringThresholds, facesPerImage map[uint]int,
) int {
	var people []domain.Person
	if err := m.db.Order("id").Find(&people).Error; err != nil {
		m.setError(fmt.Sprintf("load people: %v", err))
		return 0
	}
	// A manual person is a user decision: reconciliation neither merges it away
	// nor merges another person into it.
	considered := make([]domain.Person, 0, len(people))
	for _, person := range people {
		if person.Manual {
			continue
		}
		considered = append(considered, person)
	}
	people = considered
	if len(people) == 0 {
		return 0
	}

	medoids := make(map[uint][]float32)
	byPerson := make(map[uint][][]float32)
	for _, person := range people {
		vectors := m.personVectors(person.ID)
		if len(vectors) == 0 {
			m.rebuildReferences(person.ID, facesPerImage, thresholds)
			vectors = m.personVectors(person.ID)
		}
		if len(vectors) == 0 {
			continue
		}
		byPerson[person.ID] = vectors
		medoids[person.ID] = meanVector(vectors)
	}

	ids := make([]uint, 0, len(medoids))
	for personID := range medoids {
		ids = append(ids, personID)
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })

	// Size of every person bounds a single merge: a person may not grow past a
	// plausible share of the gallery. This is a safety valve, not the splitting
	// mechanism — without it one lookalike chain can swallow the library.
	type countRow struct {
		PersonID uint
		Count    int
	}
	var counts []countRow
	if err := m.db.Model(&domain.Face{}).Select("person_id, count(*) AS count").
		Where("person_id > 0").Group("person_id").Scan(&counts).Error; err != nil {
		m.setError(fmt.Sprintf("count person faces: %v", err))
		return 0
	}
	sizes := make(map[uint]int, len(counts))
	assignedFaces := 0
	for _, row := range counts {
		sizes[row.PersonID] = row.Count
		assignedFaces += row.Count
	}
	maxPersonFaces := int(float64(assignedFaces) * reconcileMaxShare)
	if maxPersonFaces < reconcileMaxFacesMin {
		maxPersonFaces = reconcileMaxFacesMin
	}

	// Each person keeps its single best partner; only mutual bests merge. A
	// transitive union over every qualifying pair turns a chain of lookalikes
	// into one gallery-wide person, which is exactly the failure this change
	// removes.
	type match struct {
		score float64
		other uint
	}
	best := make(map[uint]match, len(ids))
	for leftIndex := 0; leftIndex < len(ids); leftIndex++ {
		for rightIndex := leftIndex + 1; rightIndex < len(ids); rightIndex++ {
			left, right := ids[leftIndex], ids[rightIndex]
			if sizes[left]+sizes[right] > maxPersonFaces {
				continue
			}
			score := cosineSimilarity(medoids[left], medoids[right])
			if score < thresholds.merge {
				continue
			}
			if crossSupport(byPerson[left], byPerson[right], thresholds.supportSimilarity) < thresholds.mergeSupport {
				continue
			}
			if current, ok := best[left]; !ok || score > current.score {
				best[left] = match{score: score, other: right}
			}
			if current, ok := best[right]; !ok || score > current.score {
				best[right] = match{score: score, other: left}
			}
		}
	}

	handled := make(map[uint]bool, len(ids))
	merged := 0
	for _, personID := range ids {
		partner := best[personID].other
		if partner == 0 || handled[personID] || handled[partner] {
			continue
		}
		if best[partner].other != personID {
			continue
		}
		handled[personID], handled[partner] = true, true

		survivor, mergedAway := personID, partner
		if mergedAway < survivor {
			survivor, mergedAway = mergedAway, survivor
		}
		if err := m.db.Model(&domain.Face{}).
			Where("person_id = ?", mergedAway).
			Update("person_id", survivor).Error; err != nil {
			m.setError(fmt.Sprintf("merge faces into person %d: %v", survivor, err))
		}
		if err := m.db.Where("id = ?", mergedAway).Delete(&domain.Person{}).Error; err != nil {
			m.setError(fmt.Sprintf("delete merged person %d: %v", mergedAway, err))
		}
		if err := m.personRefs.DeleteByPersonIDs([]uint{mergedAway}); err != nil {
			m.setError(fmt.Sprintf("delete merged references: %v", err))
		}
		if err := m.db.Model(&domain.Person{}).Where("id = ?", survivor).
			Update("cluster_version", personClusterVersion).Error; err != nil {
			m.setError(fmt.Sprintf("mark person %d reconciled: %v", survivor, err))
		}
		m.rebuildReferences(survivor, facesPerImage, thresholds)
		merged++
		log.Printf("Face recognition: reconciled person %d into person %d", mergedAway, survivor)
	}
	return merged
}

// personVectors returns the exemplar vectors currently stored for a person.
func (m *Manager) personVectors(personID uint) [][]float32 {
	if m.personRefs == nil {
		return nil
	}
	references, err := m.personRefs.List(personID)
	if err != nil {
		m.setError(fmt.Sprintf("list references of person %d: %v", personID, err))
		return nil
	}
	vectors := make([][]float32, 0, len(references))
	for _, reference := range references {
		vector, err := domain.ParseEmbedding(reference.Embedding)
		if err != nil || len(vector) == 0 {
			continue
		}
		vectors = append(vectors, vector)
	}
	return vectors
}

// assignFacesToPerson updates the person of several faces in batches.
func (m *Manager) assignFacesToPerson(personID uint, faceIDs []uint) {
	if len(faceIDs) == 0 {
		return
	}
	for start := 0; start < len(faceIDs); start += assignmentBatch {
		end := min(start+assignmentBatch, len(faceIDs))
		if err := m.db.Model(&domain.Face{}).
			Where("id IN ?", faceIDs[start:end]).
			Update("person_id", personID).Error; err != nil {
			m.setError(fmt.Sprintf("assign faces to person %d: %v", personID, err))
		}
	}
}

// meanVector returns the normalised centroid of *vectors*.
func meanVector(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}
	dimension := 0
	for _, vector := range vectors {
		if len(vector) > dimension {
			dimension = len(vector)
		}
	}
	centroid := make([]float32, dimension)
	for _, vector := range vectors {
		for index, value := range vector {
			centroid[index] += value
		}
	}
	normalise(centroid)
	return centroid
}

func normalise(vector []float32) {
	var norm float64
	for _, value := range vector {
		norm += float64(value) * float64(value)
	}
	if norm == 0 {
		return
	}
	scale := float32(1 / math.Sqrt(norm))
	for index := range vector {
		vector[index] *= scale
	}
}

// bestSimilarity is the strongest cosine similarity between *vector* and the
// exemplar set.
func bestSimilarity(vector []float32, references [][]float32) float64 {
	best := -1.0
	for _, reference := range references {
		if score := cosineSimilarity(vector, reference); score > best {
			best = score
		}
	}
	return best
}

// crossSupport counts exemplar pairs of two people that clear *floor*.
func crossSupport(left, right [][]float32, floor float64) int {
	count := 0
	for _, a := range left {
		for _, b := range right {
			if cosineSimilarity(a, b) >= floor {
				count++
			}
		}
	}
	return count
}

func cosineSimilarity(left, right []float32) float64 {
	length := min(len(left), len(right))
	if length == 0 {
		return 0
	}
	var dot, leftNorm, rightNorm float64
	for index := 0; index < length; index++ {
		a, b := float64(left[index]), float64(right[index])
		dot += a * b
		leftNorm += a * a
		rightNorm += b * b
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}
