package handler

import (
	"net/http"
	"strings"

	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
)

// handleMoveFiles moves selected files to a target directory.
// Name conflicts are resolved by appending a numeric suffix.
func (s *Server) handleMoveFiles(c *gin.Context) {
	var req dto.MoveFilesRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if len(req.FilePaths) == 0 {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgMoveFilesNoFilesSelected))
		return
	}

	if strings.TrimSpace(req.TargetDir) == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgMoveFilesTargetDirRequired))
		return
	}

	result := s.fileMover.BatchMoveFiles(req.FilePaths, req.TargetDir)

	c.JSON(http.StatusOK, dto.MoveFilesResponse{
		Success:     result.Success,
		Failed:      result.Failed,
		FailedFiles: result.FailedFiles,
	})
}
