package database

import (
	"context"
	"os"
	"testing"

	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestFaceNeighboursUnsupportedOnSQLite guards the documented fallback: unit
// tests run on SQLite, where nearest-neighbour queries cannot be answered.
func TestFaceNeighboursUnsupportedOnSQLite(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	if NewFaceNeighbours(db).Supported(context.Background()) {
		t.Fatal("SQLite must report the neighbour source as unsupported")
	}
}

// TestEnsureFaceEmbeddingIndexNoopOnSQLite guards that the index step degrades
// quietly where it cannot be built.
func TestEnsureFaceEmbeddingIndexNoopOnSQLite(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	created, err := EnsureFaceEmbeddingIndex(context.Background(), db)
	if err != nil {
		t.Fatalf("EnsureFaceEmbeddingIndex: %v", err)
	}
	if created {
		t.Fatal("SQLite must report that no face embedding index was created")
	}
}

// TestFaceNeighboursEnsuresIndex guards the port wiring: the neighbour source
// doubles as the index maintainer.
func TestFaceNeighboursEnsuresIndex(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	created, err := NewFaceNeighbours(db).EnsureNeighbourIndex(context.Background())
	if err != nil {
		t.Fatalf("EnsureNeighbourIndex: %v", err)
	}
	if created {
		t.Fatal("SQLite must report that no neighbour index was created")
	}
}

// TestFaceNeighboursQuery exercises the real lateral HNSW probe. It needs a
// PostgreSQL server with pgvector 0.7+ and is skipped unless FACES_TEST_PG_DSN
// is set.
func TestFaceNeighboursQuery(t *testing.T) {
	dsn := os.Getenv("FACES_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set FACES_TEST_PG_DSN to run the pgvector neighbour query")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	// Pin a single connection so the scratch schema below stays in effect for
	// the neighbour query as well.
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()

	const schema = "faces_knn_test"
	exec := func(query string) {
		t.Helper()
		if err := db.Exec(query).Error; err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE")
	exec("CREATE SCHEMA " + schema)
	defer db.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE")
	exec("SET search_path TO " + schema)
	exec("CREATE TABLE face_embeddings (face_id bigint NOT NULL, embedding halfvec(3) NOT NULL)")
	exec(`INSERT INTO face_embeddings (face_id, embedding) VALUES
		(1, '[1,0,0]'::halfvec(3)),
		(2, '[0.9,0.1,0]'::halfvec(3)),
		(3, '[0,1,0]'::halfvec(3))`)

	source := NewFaceNeighbours(db)
	if !source.Supported(context.Background()) {
		t.Skip("pgvector on this server does not support halfvec")
	}

	grouped, err := source.Neighbours(context.Background(), []uint{1, 2, 3}, 0.4, 5)
	if err != nil {
		t.Fatalf("Neighbours: %v", err)
	}

	// Face 1 and face 2 are at cosine 0.9; face 3 is orthogonal to both.
	if len(grouped[1]) != 1 || grouped[1][0].FaceID != 2 {
		t.Fatalf("neighbours of face 1 = %+v, want only face 2", grouped[1])
	}
	if grouped[1][0].Similarity < 0.85 || grouped[1][0].Similarity > 0.95 {
		t.Errorf("similarity(1,2) = %f, want about 0.9", grouped[1][0].Similarity)
	}
	if len(grouped[2]) != 1 || grouped[2][0].FaceID != 1 {
		t.Errorf("neighbours of face 2 = %+v, want only face 1", grouped[2])
	}
	if len(grouped[3]) != 0 {
		t.Errorf("neighbours of face 3 = %+v, want none inside the radius", grouped[3])
	}
}
