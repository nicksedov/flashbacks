package imaging

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"math"
	"os"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/ocr"

	"github.com/deepteams/webp"
	"github.com/disintegration/imaging"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Batch helpers (from ocr_batch.go)
// ---------------------------------------------------------------------------

const batchSize = 20

// ClassificationBatch holds pending classifications and their bounding boxes for batch saving
type ClassificationBatch struct {
	Classifications []domain.OcrClassification
	BoxesByImage    map[uint][]domain.OcrBoundingBox
	db              *gorm.DB
}

// NewClassificationBatch creates a new empty batch
func NewClassificationBatch(db *gorm.DB) *ClassificationBatch {
	return &ClassificationBatch{
		BoxesByImage: make(map[uint][]domain.OcrBoundingBox),
		db:           db,
	}
}

// Add adds a classification result to the batch
func (b *ClassificationBatch) Add(result OcrResult) {
	if result.Classification == nil {
		return
	}

	b.Classifications = append(b.Classifications, *result.Classification)
	if len(result.Boxes) > 0 {
		b.BoxesByImage[result.Classification.ImageFileID] = result.Boxes
	}
}

// IsFull returns true if the batch has reached the size limit
func (b *ClassificationBatch) IsFull() bool {
	return len(b.Classifications) >= batchSize
}

// Save persists the batch to the database and resets it
func (b *ClassificationBatch) Save() {
	if len(b.Classifications) == 0 {
		return
	}

	log.Printf("[OCR] Saving batch of %d classifications", len(b.Classifications))

	for i := range b.Classifications {
		classification := &b.Classifications[i]
		if err := b.db.Where("image_file_id = ?", classification.ImageFileID).
			Assign(classification).
			FirstOrCreate(classification).Error; err != nil {
			log.Printf("OCR: failed to save classification for image %d: %v", classification.ImageFileID, err)
			continue
		}

		// Save bounding boxes only for text document classifications
		if classification.IsTextDocument {
			if boxes, ok := b.BoxesByImage[classification.ImageFileID]; ok {
				b.saveBoundingBoxes(classification.ID, boxes)
				delete(b.BoxesByImage, classification.ImageFileID)
			}
		}
	}

	// Reset batch
	b.Classifications = b.Classifications[:0]
}

// saveBoundingBoxes deletes old boxes and inserts new ones for a classification
func (b *ClassificationBatch) saveBoundingBoxes(classificationID uint, boxes []domain.OcrBoundingBox) {
	b.db.Where("classification_id = ?", classificationID).Delete(&domain.OcrBoundingBox{})

	for j := range boxes {
		boxes[j].ClassificationID = classificationID
		if err := b.db.Create(&boxes[j]).Error; err != nil {
			log.Printf("OCR: failed to save bounding box for classification %d: %v", classificationID, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Worker (from ocr_worker.go)
// ---------------------------------------------------------------------------

// OcrResult represents the result of processing a single image
type OcrResult struct {
	Image          domain.ImageFile
	Classification *domain.OcrClassification
	Boxes          []domain.OcrBoundingBox
	Err            error
}

// processSingleImage performs OCR classification on a single image file
func (om *OcrManager) processSingleImage(image domain.ImageFile, stopRequested func() bool) OcrResult {
	result := OcrResult{Image: image}

	// Check for stop before processing
	if stopRequested() {
		log.Printf("[OCR] Image ID=%d STOPPED before processing", image.ID)
		result.Err = fmt.Errorf("classification stopped")
		return result
	}

	// Open image file
	log.Printf("[OCR] Opening file: %s (image ID=%d)", image.Path, image.ID)
	file, err := os.Open(image.Path)
	if err != nil {
		log.Printf("[OCR] ERROR opening file %s (ID=%d): %v", image.Path, image.ID, err)
		result.Err = fmt.Errorf("failed to open file: %w", err)
		return result
	}
	defer file.Close()
	log.Printf("[OCR] File opened OK: %s (ID=%d)", image.Path, image.ID)

	// Determine content type based on extension
	contentType := om.detectContentType(image.Path)

	// Call OCR API
	log.Printf("[OCR] Calling OCR API for image ID=%d, path=%s, contentType=%s", image.ID, image.Path, contentType)
	ocrStart := time.Now()
	ctx := context.Background()
	ocrResp, err := om.ocrClient.Classify(ctx, file, contentType, ocr.DefaultClassifyParams())
	ocrDuration := time.Since(ocrStart)

	if err != nil {
		log.Printf("[OCR] OCR API FAILED for image ID=%d after %v: %v", image.ID, ocrDuration, err)
		result.Err = fmt.Errorf("OCR classification failed: %w", err)
		return result
	}
	log.Printf("[OCR] OCR API OK for image ID=%d in %v: isText=%v, confidence=%.3f", image.ID, ocrDuration, ocrResp.IsTextDocument, ocrResp.MeanConfidence)

	// Build classification record with transformed bounding box dimensions
	rotatedWidth, rotatedHeight := transformBoundingBoxDimensions(
		ocrResp.Angle,
		ocrResp.BoundingBoxWidth,
		ocrResp.BoundingBoxHeight,
	)

	result.Classification = &domain.OcrClassification{
		ImageFileID:        image.ID,
		IsTextDocument:     ocrResp.IsTextDocument,
		MeanConfidence:     float32(ocrResp.MeanConfidence),
		WeightedConfidence: float32(ocrResp.WeightedConfidence),
		TokenCount:         ocrResp.TokenCount,
		Angle:              ocrResp.Angle,
		ScaleFactor:        float32(ocrResp.ScaleFactor),
		BoundingBoxWidth:   rotatedWidth,
		BoundingBoxHeight:  rotatedHeight,
	}

	// Build bounding box records only for text documents
	if ocrResp.IsTextDocument && len(ocrResp.Boxes) > 0 {
		for _, box := range ocrResp.Boxes {
			result.Boxes = append(result.Boxes, domain.OcrBoundingBox{
				X:          box.X,
				Y:          box.Y,
				Width:      box.Width,
				Height:     box.Height,
				Word:       box.Word,
				Confidence: float32(box.Confidence),
			})
		}
	}

	return result
}

// detectContentType determines the MIME content type from file extension
func (om *OcrManager) detectContentType(path string) string {
	if len(path) > 4 {
		ext := path[len(path)-4:]
		if ext == ".png" {
			return "image/png"
		}
	}
	return "image/jpeg"
}

// transformBoundingBoxDimensions applies affine transformation to calculate the correct
// bounding box dimensions after counter-clockwise rotation.
func transformBoundingBoxDimensions(angle int, srcWidth, srcHeight int) (int, int) {
	if angle == 0 {
		return srcWidth, srcHeight
	}

	normalizedAngle := ((angle % 360) + 360) % 360
	angleRad := float64(normalizedAngle) * math.Pi / 180

	cos := math.Cos(angleRad)
	sin := math.Sin(angleRad)

	rotatedWidth := int(math.Abs(float64(srcWidth)*cos) + math.Abs(float64(srcHeight)*sin))
	rotatedHeight := int(math.Abs(float64(srcWidth)*sin) + math.Abs(float64(srcHeight)*cos))

	return rotatedWidth, rotatedHeight
}

// ---------------------------------------------------------------------------
// Query helpers (from ocr_query.go)
// ---------------------------------------------------------------------------

// queryUnclassifiedImages builds and executes a database query to find images that need OCR classification
func (om *OcrManager) queryUnclassifiedImages(incremental bool) ([]domain.ImageFile, error) {
	log.Printf("[OCR] Building query: incremental=%v", incremental)

	query := om.db.Table("image_files").
		Select("image_files.*").
		Joins("LEFT JOIN ocr_classifications ON ocr_classifications.image_file_id = image_files.id")

	if incremental {
		query = query.Where("ocr_classifications.id IS NULL OR ocr_classifications.updated_at < image_files.updated_at")
	} else {
		query = query.Where("1=1")
	}
	query = query.Order("image_files.id")

	log.Printf("[OCR] Executing database query...")
	queryStart := time.Now()
	var images []domain.ImageFile
	if err := query.Find(&images).Error; err != nil {
		log.Printf("[OCR] ERROR: failed to query images: %v", err)
		return nil, fmt.Errorf("failed to query images: %w", err)
	}
	log.Printf("[OCR] Database query completed in %v: found %d images", time.Since(queryStart), len(images))

	return images, nil
}

// updateProgress updates the processing progress status
func (om *OcrManager) updateProgress(filesProcessed, totalFiles int, errorMessage string) {
	om.mu.Lock()
	defer om.mu.Unlock()

	om.filesProcessed = filesProcessed
	if errorMessage != "" {
		om.progress = errorMessage
	} else {
		om.progress = fmt.Sprintf("Classifying: %d/%d", filesProcessed, totalFiles)
	}
}

// updateProgressf updates the processing progress with a formatted message
func (om *OcrManager) updateProgressf(format string, args ...interface{}) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.progress = fmt.Sprintf(format, args...)
}

// isStopRequested checks if a stop has been requested
func (om *OcrManager) isStopRequested() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.stopRequested
}

// stopRequestedFunc returns a function that checks if stop was requested (for passing to workers)
func (om *OcrManager) stopRequestedFunc() func() bool {
	return om.isStopRequested
}

// ---------------------------------------------------------------------------
// Image preparation (from ocr_image.go)
// ---------------------------------------------------------------------------

// PrepareOcrImage opens an image, scales it by scaleFactor and rotates
// clockwise by the given angle (in degrees). Returns WebP-encoded bytes.
func PrepareOcrImage(imagePath string, scaleFactor float64, angle float64) ([]byte, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image %s: %w", imagePath, err)
	}

	// Scale by scaleFactor
	if scaleFactor != 1.0 {
		newWidth := int(float64(img.Bounds().Dx()) * scaleFactor)
		newHeight := int(float64(img.Bounds().Dy()) * scaleFactor)
		if newWidth > 0 && newHeight > 0 {
			img = imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
		}
	}

	// Rotate counter-clockwise by angle
	if angle != 0 {
		img = imaging.Rotate(img, angle, color.Black)
	}

	var buf bytes.Buffer
	err = webp.Encode(&buf, img, &webp.Options{Quality: 80})
	if err != nil {
		// Fallback to JPEG if WebP encoding fails
		buf.Reset()
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return nil, fmt.Errorf("failed to encode image: %w", err)
		}
		return buf.Bytes(), nil
	}

	return buf.Bytes(), nil
}
