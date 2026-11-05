package faces

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/flashbacks/core/internal/domain"
)

// clusteringThresholds are the values the faces service advertises; the
// fallbacks keep an older service working.
type clusteringThresholds struct {
	assign            float64
	margin            float64
	merge             float64
	mergeSupport      int
	supportSimilarity float64
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

// thresholds reads the clustering thresholds from the faces service.
func (m *Manager) thresholds(ctx context.Context) clusteringThresholds {
	resolved := clusteringThresholds{
		assign:            fallbackAssignmentSimilarity,
		margin:            fallbackRecognitionMargin,
		merge:             fallbackMergeSimilarity,
		mergeSupport:      fallbackMergeSupport,
		supportSimilarity: fallbackMergeSupportSimilarity,
	}
	health, err := m.client.Health(ctx)
	if err != nil {
		log.Printf("Face recognition: read faces health: %v", err)
		return resolved
	}
	if value := health.Thresholds.ClusterAssignSimilarity; value > 0 {
		resolved.assign = value
	}
	if value := health.Thresholds.RecognitionMargin; value > 0 {
		resolved.margin = value
	}
	if value := health.Thresholds.ClusterMergeSimilarity; value > 0 {
		resolved.merge = value
	}
	if value := health.Thresholds.ClusterMergeSupport; value > 0 {
		resolved.mergeSupport = value
	}
	if value := health.Thresholds.ClusterMergeSupportSimilarity; value > 0 {
		resolved.supportSimilarity = value
	}
	return resolved
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
func (m *Manager) replaceReferences(
	personID uint,
	cluster []faceRef,
	memberVectors map[uint][]float32,
	facesPerImage map[uint]int,
) {
	if m.personRefs == nil {
		return
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
func (m *Manager) rebuildReferences(personID uint, facesPerImage map[uint]int) {
	var faces []domain.Face
	if err := m.db.Where("person_id = ?", personID).Order("confidence DESC, id").
		Find(&faces).Error; err != nil {
		m.setError(fmt.Sprintf("load faces of person %d: %v", personID, err))
		return
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
		})
		memberVectors[face.ID] = vector
	}
	m.replaceReferences(personID, cluster, memberVectors, facesPerImage)
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
	thresholds := m.thresholds(ctx)
	facesPerImage := m.imageFaceCounts()

	for m.reconcileRound(thresholds, facesPerImage) > 0 {
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
	if len(people) == 0 {
		return 0
	}

	medoids := make(map[uint][]float32)
	byPerson := make(map[uint][][]float32)
	for _, person := range people {
		vectors := m.personVectors(person.ID)
		if len(vectors) == 0 {
			m.rebuildReferences(person.ID, facesPerImage)
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

	parent := make(map[uint]uint, len(ids))
	for _, personID := range ids {
		parent[personID] = personID
	}
	var find func(uint) uint
	find = func(personID uint) uint {
		for parent[personID] != personID {
			parent[personID] = parent[parent[personID]]
			personID = parent[personID]
		}
		return personID
	}
	union := func(left, right uint) {
		leftRoot, rightRoot := find(left), find(right)
		if leftRoot != rightRoot {
			parent[rightRoot] = leftRoot
		}
	}

	for leftIndex := 0; leftIndex < len(ids); leftIndex++ {
		for rightIndex := leftIndex + 1; rightIndex < len(ids); rightIndex++ {
			left, right := ids[leftIndex], ids[rightIndex]
			if cosineSimilarity(medoids[left], medoids[right]) < thresholds.merge {
				continue
			}
			if crossSupport(byPerson[left], byPerson[right], thresholds.supportSimilarity) < thresholds.mergeSupport {
				continue
			}
			union(left, right)
		}
	}

	groups := make(map[uint][]uint)
	for _, personID := range ids {
		root := find(personID)
		groups[root] = append(groups[root], personID)
	}

	merged := 0
	for _, group := range groups {
		survivor := group[0]
		for _, personID := range group[1:] {
			if personID < survivor {
				survivor = personID
			}
		}
		mergedAway := make([]uint, 0, len(group)-1)
		for _, personID := range group {
			if personID != survivor {
				mergedAway = append(mergedAway, personID)
			}
		}
		if len(mergedAway) == 0 {
			m.db.Model(&domain.Person{}).Where("id = ?", survivor).
				Update("cluster_version", personClusterVersion)
			continue
		}

		for start := 0; start < len(mergedAway); start += assignmentBatch {
			end := min(start+assignmentBatch, len(mergedAway))
			if err := m.db.Model(&domain.Face{}).
				Where("person_id IN ?", mergedAway[start:end]).
				Update("person_id", survivor).Error; err != nil {
				m.setError(fmt.Sprintf("merge faces into person %d: %v", survivor, err))
			}
		}
		if err := m.db.Where("id IN ?", mergedAway).Delete(&domain.Person{}).Error; err != nil {
			m.setError(fmt.Sprintf("delete merged people: %v", err))
		}
		if err := m.personRefs.DeleteByPersonIDs(mergedAway); err != nil {
			m.setError(fmt.Sprintf("delete merged references: %v", err))
		}
		if err := m.db.Model(&domain.Person{}).Where("id = ?", survivor).
			Update("cluster_version", personClusterVersion).Error; err != nil {
			m.setError(fmt.Sprintf("mark person %d reconciled: %v", survivor, err))
		}
		m.rebuildReferences(survivor, facesPerImage)
		merged += len(mergedAway)
		log.Printf("Face recognition: reconciled %d people into person %d", len(group), survivor)
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
