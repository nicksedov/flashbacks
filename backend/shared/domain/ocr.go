package domain

// BoundingBox represents a detected text region with its position and confidence.
// This is the canonical definition shared across OCR and API services.
type BoundingBox struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Word       string  `json:"word"`
	Confidence float64 `json:"confidence"`
}

// ClassifyResponse contains the results of OCR text detection on an image.
// This is the canonical definition shared across OCR and API services.
type ClassifyResponse struct {
	MeanConfidence     float64       `json:"mean_confidence"`
	WeightedConfidence float64       `json:"weighted_confidence"`
	TokenCount         int           `json:"token_count"`
	Boxes              []BoundingBox `json:"boxes"`
	Angle              int           `json:"angle"`
	ScaleFactor        float64       `json:"scale_factor"`
	IsTextDocument     bool          `json:"is_text_document"`
	BoundingBoxWidth   int           `json:"bounding_box_width"`
	BoundingBoxHeight  int           `json:"bounding_box_height"`
}
