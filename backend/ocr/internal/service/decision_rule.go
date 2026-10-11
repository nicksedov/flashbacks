package service

const (
	// DefaultMinConfidence is the minimum weighted confidence required by the
	// primary decision rule.
	DefaultMinConfidence = 0.66
	// DefaultMinTokenCount is the minimum number of recognised tokens required
	// by the primary decision rule.
	DefaultMinTokenCount = 20

	// DenseTextMinTokenCount is the token count from which a document is
	// considered to contain a large amount of text.
	DenseTextMinTokenCount = 300

	// DenseTextTrimFraction is the share of the lowest-confidence tokens that is
	// discarded before the dense-text confidence is measured. Degraded scans
	// carry a noisy low-confidence tail that drags the plain average down;
	// trimming it exposes the confidence of the bulk of the recognised text.
	DenseTextTrimFraction = 0.20

	// DenseTextMinConfidence is the confidence floor applied to the trimmed
	// dense-text confidence (ClassifierResult.DenseTextConfidence).
	//
	// Because the noisiest DenseTextTrimFraction of the tokens is removed first,
	// this floor is deliberately higher than the plain weighted-confidence
	// average of a degraded scan. Measured against the OCR dataset the valid
	// range is (0.472, 0.516]; 0.50 sits inside it with margin on both sides.
	DenseTextMinConfidence = 0.50
)

// DecisionRule holds the criteria for determining if a document is a text document.
type DecisionRule struct {
	MinConfidence float64 // Minimum weighted confidence (0-1)
	MinTokenCount int     // Minimum token count
	// OCRParams holds OCR-specific parameters (optional).
	// If empty defaults will be used: Language="eng+rus", Level=RIL_WORD
	OCRParams
}

// GetDefaultDecisionRule returns the default decision criteria.
func GetDefaultDecisionRule() DecisionRule {
	level := DefaultPageIteratorLevel
	return DecisionRule{
		MinConfidence: DefaultMinConfidence,
		MinTokenCount: DefaultMinTokenCount,
		OCRParams: OCRParams{
			Language: DefaultLanguage,
			Level:    &level,
		},
	}
}

// EvaluateDecision determines if a classification result represents a text document.
//
// A document qualifies when it contains at least MinTokenCount tokens and either:
//
//   - the weighted confidence reaches MinConfidence, or
//   - it is a dense-text document (at least DenseTextMinTokenCount tokens) whose
//     trimmed confidence (DenseTextConfidence, measured after dropping the
//     lowest-confidence DenseTextTrimFraction of tokens) reaches
//     DenseTextMinConfidence.
//
// The second branch exists because degraded scans yield a large amount of text
// at reduced average confidence; deciding on the full weighted confidence
// rejects them even though the bulk of the recognised text is confident.
func EvaluateDecision(result *ClassifierResult, rule DecisionRule) bool {
	if result.TokenCount < rule.MinTokenCount {
		return false
	}

	if result.WeightedConfidence >= rule.MinConfidence {
		return true
	}

	return result.TokenCount >= DenseTextMinTokenCount &&
		result.DenseTextConfidence >= DenseTextMinConfidence
}

// EvaluateDecisionWithDefaults uses default criteria to evaluate the decision.
func EvaluateDecisionWithDefaults(result *ClassifierResult) bool {
	return EvaluateDecision(result, GetDefaultDecisionRule())
}
