package faces

// Temporary snapshot harness for task 5.2: runs the reconcile step twice
// against a copy of the production gallery and prints the before/after metrics.
// It is deleted after the report is recorded and never runs in CI (skipped
// unless SNAPSHOT_DSN is set).

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	facesclient "github.com/flashbacks/core/internal/infrastructure/faces"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type snapshotMetrics struct {
	Faces         int64   `json:"faces"`
	People        int64   `json:"people"`
	Unassigned    int64   `json:"unassigned"`
	UnassignedPct float64 `json:"unassignedPct"`
	DimaPeople    int64   `json:"dimaPeople"`
	DimaLargest   int64   `json:"dimaLargest"`
	DimaAssigned  int64   `json:"dimaAssigned"`
	References    int64   `json:"references"`
}

func snapshotStats(t *testing.T, db *gorm.DB) snapshotMetrics {
	t.Helper()
	var metrics snapshotMetrics
	if err := db.Model(&domain.Face{}).Count(&metrics.Faces).Error; err != nil {
		t.Fatalf("count faces: %v", err)
	}
	if err := db.Model(&domain.Person{}).Count(&metrics.People).Error; err != nil {
		t.Fatalf("count people: %v", err)
	}
	if err := db.Model(&domain.Face{}).Where("person_id = 0").Count(&metrics.Unassigned).Error; err != nil {
		t.Fatalf("count unassigned: %v", err)
	}
	if metrics.Faces > 0 {
		metrics.UnassignedPct = float64(metrics.Unassigned) * 100 / float64(metrics.Faces)
	}
	if err := db.Raw(
		`SELECT count(DISTINCT f.person_id) FROM faces f
		 JOIN dima_faces d ON d.id = f.id WHERE f.person_id > 0`,
	).Scan(&metrics.DimaPeople).Error; err != nil {
		t.Fatalf("count dima people: %v", err)
	}
	if err := db.Raw(
		`SELECT coalesce(max(c), 0) FROM (
		   SELECT count(*) AS c FROM faces f
		   JOIN dima_faces d ON d.id = f.id
		   WHERE f.person_id > 0 GROUP BY f.person_id) s`,
	).Scan(&metrics.DimaLargest).Error; err != nil {
		t.Fatalf("count dima largest: %v", err)
	}
	if err := db.Raw(
		`SELECT count(*) FROM faces f JOIN dima_faces d ON d.id = f.id WHERE f.person_id > 0`,
	).Scan(&metrics.DimaAssigned).Error; err != nil {
		t.Fatalf("count dima assigned: %v", err)
	}
	if err := db.Model(&domain.PersonReference{}).Count(&metrics.References).Error; err != nil {
		t.Fatalf("count references: %v", err)
	}
	return metrics
}

func TestSnapshotReconcile(t *testing.T) {
	dsn := os.Getenv("SNAPSHOT_DSN")
	if dsn == "" {
		t.Skip("SNAPSHOT_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	if err := db.AutoMigrate(&domain.Person{}, &domain.PersonReference{}); err != nil {
		t.Fatalf("migrate snapshot: %v", err)
	}

	// The old faces service is enough for the reconcile step: it only reads
	// thresholds, and an unreachable client exercises the calibrated fallbacks.
	client := facesclient.NewClient("http://127.0.0.1:1")
	manager := NewManagerWithDependencies(db, client, Dependencies{
		PersonReferences: repository.NewPersonReferenceRepository(db),
	}, DefaultOptions())

	before := snapshotStats(t, db)

	started := time.Now()
	manager.reconcilePeople()
	firstElapsed := time.Since(started)
	afterFirst := snapshotStats(t, db)

	started = time.Now()
	manager.reconcilePeople()
	secondElapsed := time.Since(started)
	afterSecond := snapshotStats(t, db)

	report := map[string]any{
		"before":        before,
		"afterFirst":    afterFirst,
		"afterSecond":   afterSecond,
		"firstSeconds":  firstElapsed.Seconds(),
		"secondSeconds": secondElapsed.Seconds(),
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("encode report: %v", err)
	}
	fmt.Println("SNAPSHOT_REPORT " + string(encoded))
}
