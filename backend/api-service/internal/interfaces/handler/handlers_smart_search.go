package handler

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

// smartSearchImageDTO represents a single result in the smart search response.
type smartSearchImageDTO struct {
	ID         uint              `json:"id"`
	Path       string            `json:"path"`
	FileName   string            `json:"fileName"`
	ModTime    string            `json:"modTime,omitempty"`
	Similarity float64           `json:"similarity"`
	Tags       []string          `json:"tags"`
	MatchType  imaging.MatchType `json:"matchType,omitempty"`
}

// smartSearchResponse is the response for the smart search endpoint.
type smartSearchResponse struct {
	Images []smartSearchImageDTO `json:"images"`
	Total  int                   `json:"total"`
	Query  string                `json:"query"`
}

// handleSmartSearch performs semantic search over image tag embeddings
// combined with exact tag matching. Both searches run in parallel.
func (s *Server) handleSmartSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgSmartQueryRequired)
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	// Run exact tag search and embedding search in parallel
	var exactResults []imaging.SmartSearchResult
	var embeddingResult imaging.SmartSearchResponse

	g, ctx := errgroup.WithContext(c.Request.Context())

	g.Go(func() error {
		var err error
		exactResults, err = imaging.SearchByExactTag(s.db, query)
		return err
	})

	g.Go(func() error {
		// Use context-aware wrapper, but SearchByEmbedding currently doesn't
		// support context cancellation. Pass the limit for embedding search.
		var err error
		embeddingResult, err = imaging.SearchByEmbedding(s.db, query, limit)
		return err
	})

	if err := g.Wait(); err != nil {
		s.respondError(c, http.StatusServiceUnavailable, i18n.MsgSmartSearchFailed)
		return
	}

	// Merge and rank results from both streams
	finalResult := imaging.MergeAndRankResults(exactResults, embeddingResult.Images)
	finalResult.Query = query

	// Slice to requested limit
	if len(finalResult.Images) > limit {
		finalResult.Images = finalResult.Images[:limit]
		finalResult.Total = limit
	}

	images := make([]smartSearchImageDTO, 0, len(finalResult.Images))
	for _, img := range finalResult.Images {
		images = append(images, smartSearchImageDTO{
			ID:         img.ImageFileID,
			Path:       img.Path,
			FileName:   filepath.Base(img.Path),
			ModTime:    img.ModTime.Format("2006-01-02 15:04:05"),
			Similarity: img.Similarity,
			Tags:       img.Tags,
			MatchType:  img.MatchType,
		})
	}

	// Ignore ctx.Err from errgroup context since we don't pass it to SearchByEmbedding yet
	_ = ctx

	c.JSON(http.StatusOK, smartSearchResponse{
		Images: images,
		Total:  len(images),
		Query:  query,
	})
}
