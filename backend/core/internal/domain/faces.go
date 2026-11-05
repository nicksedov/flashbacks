package domain

import "time"

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
	CoverFaceID *uint     `json:"coverFaceId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
