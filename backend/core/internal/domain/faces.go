package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FaceEmbeddingModel identifies the ArcFace model that produced an embedding.
const FaceEmbeddingModel = "arcface_w600k_r50"

// Face clustering strategies selected by FACES_CLUSTER_MODE.
const (
	// FacesClusterModeAuto asks the vector store for candidates when it can
	// answer nearest-neighbour queries, and clusters in memory otherwise.
	FacesClusterModeAuto = "auto"
	// FacesClusterModeStore always asks the vector store for candidates.
	FacesClusterModeStore = "store"
	// FacesClusterModeMemory keeps the in-memory all-pairs call (rollback lever).
	FacesClusterModeMemory = "memory"
)

// Person represents a cluster of faces that identify one (unnamed) person.
type Person struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Name is empty until the user renames the person; the UI falls back to a
	// localized "Person N" label.
	Name string `gorm:"default:''" json:"name"`
	// CoverFaceID points at the face used as this person's cover thumbnail.
	CoverFaceID *uint `json:"coverFaceId,omitempty"`
	// ClusterVersion marks the clustering generation that produced this
	// person. A mismatch triggers the reconcile step, which merges people that
	// one identity ended up spread over.
	ClusterVersion string    `gorm:"default:''" json:"clusterVersion"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TableName overrides the default GORM table name.
func (Person) TableName() string { return "people" }

// Face is a detected face on an image. A face references exactly one person
// once the background job has clustered it (PersonID == 0 means unassigned).
type Face struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ImageFileID uint      `gorm:"not null;index" json:"imageFileId"`
	PersonID    uint      `gorm:"not null;index;default:0" json:"personId"`
	X           float32   `json:"x"`
	Y           float32   `json:"y"`
	Width       float32   `json:"width"`
	Height      float32   `json:"height"`
	Confidence  float32   `json:"confidence"`
	EmbeddingID uint      `gorm:"default:0" json:"embeddingId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TableName overrides the default GORM table name.
func (Face) TableName() string { return "faces" }

// PersonReference is one exemplar embedding of a person.
//
// A person keeps up to five exemplars (different ages, beards, lighting) so a
// later clustering pass can recognise the identity without re-clustering the
// whole gallery: compares against a set of modes instead of one blurry
// centroid. The medoid of the set is the reference used for merge decisions.
type PersonReference struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PersonID  uint      `gorm:"not null;index;uniqueIndex:idx_person_references_person_face" json:"personId"`
	FaceID    uint      `gorm:"not null;uniqueIndex:idx_person_references_person_face" json:"faceId"`
	Dimension int       `gorm:"not null" json:"dimension"`
	Embedding string    `gorm:"type:halfvec(512);not null" json:"-"`
	Quality   float32   `gorm:"default:0" json:"quality"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName overrides the default GORM table name.
func (PersonReference) TableName() string { return "person_references" }

// ParseEmbedding parses a pgvector literal ("[0.1,0.2]") into a float32 vector.
func ParseEmbedding(value string) ([]float32, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if trimmed == "" {
		return nil, nil
	}
	parts := strings.Split(trimmed, ",")
	vector := make([]float32, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
		if err != nil {
			return nil, fmt.Errorf("parse embedding component %q: %w", part, err)
		}
		vector = append(vector, float32(number))
	}
	return vector, nil
}

// FormatEmbedding renders a float32 vector as a pgvector literal.
func FormatEmbedding(vector []float32) string {
	parts := make([]string, len(vector))
	for index, value := range vector {
		parts[index] = strconv.FormatFloat(float64(value), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// FaceEmbedding stores the pgvector halfvec embedding of a detected face.
//
// The column is declared with its dimension on purpose: pgvector's HNSW index
// rejects an unconstrained “halfvec“ column ("column does not have
// dimensions"), and the kNN candidate query depends on that index.
type FaceEmbedding struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	FaceID    uint      `gorm:"not null;index" json:"faceId"`
	Dimension int       `gorm:"not null" json:"dimension"`
	Embedding string    `gorm:"type:halfvec(512);not null" json:"-"`
	Model     string    `gorm:"default:''" json:"model"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName overrides the default GORM table name.
func (FaceEmbedding) TableName() string { return "face_embeddings" }
