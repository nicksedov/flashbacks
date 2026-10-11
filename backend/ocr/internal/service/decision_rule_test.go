package service

import (
	"math"
	"testing"
)

func TestGetDefaultDecisionRule(t *testing.T) {
	rule := GetDefaultDecisionRule()

	if rule.MinConfidence != DefaultMinConfidence {
		t.Errorf("MinConfidence = %v, want %v", rule.MinConfidence, DefaultMinConfidence)
	}
	if rule.MinTokenCount != DefaultMinTokenCount {
		t.Errorf("MinTokenCount = %v, want %v", rule.MinTokenCount, DefaultMinTokenCount)
	}
	if rule.Language != DefaultLanguage {
		t.Errorf("Language = %q, want %q", rule.Language, DefaultLanguage)
	}
	if rule.Level == nil {
		t.Fatal("Level is nil, want non-nil")
	}
	if *rule.Level != DefaultPageIteratorLevel {
		t.Errorf("Level = %v, want %v", *rule.Level, DefaultPageIteratorLevel)
	}
}

func TestEvaluateDecision(t *testing.T) {
	def := GetDefaultDecisionRule()

	tests := []struct {
		name   string
		result ClassifierResult
		rule   DecisionRule
		want   bool
	}{
		{
			name:   "high confidence, enough tokens",
			result: ClassifierResult{WeightedConfidence: 0.90, TokenCount: 100},
			rule:   def,
			want:   true,
		},
		{
			name:   "exactly at confidence and token thresholds",
			result: ClassifierResult{WeightedConfidence: DefaultMinConfidence, TokenCount: DefaultMinTokenCount},
			rule:   def,
			want:   true,
		},
		{
			name:   "below token threshold rejects even high confidence",
			result: ClassifierResult{WeightedConfidence: 0.99, TokenCount: DefaultMinTokenCount - 1},
			rule:   def,
			want:   false,
		},
		{
			name:   "enough tokens but low confidence and not dense",
			result: ClassifierResult{WeightedConfidence: 0.50, DenseTextConfidence: 0.60, TokenCount: 100},
			rule:   def,
			want:   false,
		},
		{
			name:   "dense text with trimmed confidence above floor",
			result: ClassifierResult{WeightedConfidence: 0.47, DenseTextConfidence: 0.70, TokenCount: 1217},
			rule:   def,
			want:   true,
		},
		{
			name:   "dense text exactly at trimmed confidence floor",
			result: ClassifierResult{WeightedConfidence: 0.40, DenseTextConfidence: DenseTextMinConfidence, TokenCount: DenseTextMinTokenCount},
			rule:   def,
			want:   true,
		},
		{
			name:   "dense text just below trimmed confidence floor",
			result: ClassifierResult{WeightedConfidence: 0.40, DenseTextConfidence: DenseTextMinConfidence - 0.01, TokenCount: 2000},
			rule:   def,
			want:   false,
		},
		{
			name:   "just below dense token floor",
			result: ClassifierResult{WeightedConfidence: 0.40, DenseTextConfidence: 0.90, TokenCount: DenseTextMinTokenCount - 1},
			rule:   def,
			want:   false,
		},
		{
			name:   "photo texture tokens, trimmed confidence too low",
			result: ClassifierResult{WeightedConfidence: 0.435, DenseTextConfidence: 0.44, TokenCount: 240},
			rule:   def,
			want:   false,
		},
		{
			name:   "custom strict confidence threshold is honoured",
			result: ClassifierResult{WeightedConfidence: 0.60, DenseTextConfidence: 0.90, TokenCount: 100},
			rule:   DecisionRule{MinConfidence: 0.70, MinTokenCount: 20},
			want:   false,
		},
		{
			name:   "custom high token threshold blocks dense fallback",
			result: ClassifierResult{WeightedConfidence: 0.50, DenseTextConfidence: 0.95, TokenCount: 2000},
			rule:   DecisionRule{MinConfidence: 0.90, MinTokenCount: 5000},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateDecision(&tt.result, tt.rule)
			if got != tt.want {
				t.Errorf("EvaluateDecision(%+v, %+v) = %v, want %v", tt.result, tt.rule, got, tt.want)
			}
		})
	}
}

func TestEvaluateDecisionWithDefaults(t *testing.T) {
	if !EvaluateDecisionWithDefaults(&ClassifierResult{WeightedConfidence: 0.9, TokenCount: 100}) {
		t.Error("expected high-confidence, dense input to be a text document")
	}
	if EvaluateDecisionWithDefaults(&ClassifierResult{WeightedConfidence: 0.2, TokenCount: 5}) {
		t.Error("expected sparse, low-confidence input not to be a text document")
	}
}

func TestComputeDenseTextConfidence(t *testing.T) {
	tests := []struct {
		name       string
		boxes      []BoundingBox
		totalToken int
		want       float64
	}{
		{
			name:       "empty boxes",
			boxes:      nil,
			totalToken: 0,
			want:       0,
		},
		{
			name:       "drops the lowest 20% of single-token boxes",
			boxes:      singleTokenBoxes(0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0),
			totalToken: 10,
			want:       0.65, // 0.1 and 0.2 dropped → mean(0.3..1.0)
		},
		{
			name: "trimming is weighted by token count",
			boxes: []BoundingBox{
				{Word: "ab", Confidence: 0.20},       // 2 tokens (lowest)
				{Word: "abcdefgh", Confidence: 0.90}, // 8 tokens
			},
			totalToken: 10,
			want:       0.90, // the 2 low-confidence tokens are dropped entirely
		},
		{
			name:       "no tokens dropped when budget rounds to zero",
			boxes:      singleTokenBoxes(0.30, 0.70),
			totalToken: 2, // floor(2 * 0.20) = 0
			want:       0.50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDenseTextConfidence(tt.boxes, tt.totalToken)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("computeDenseTextConfidence() = %v, want %v", got, tt.want)
			}
		})
	}
}

func singleTokenBoxes(confidences ...float64) []BoundingBox {
	boxes := make([]BoundingBox, len(confidences))
	for i, c := range confidences {
		boxes[i] = BoundingBox{Word: "x", Confidence: c}
	}
	return boxes
}
