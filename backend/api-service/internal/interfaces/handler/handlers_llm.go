package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// handleGetLlmSettings returns LLM settings with all providers
func (s *Server) handleGetLlmSettings(c *gin.Context) {
	settings := s.settingsLoader.LlmSettings()
	providers := s.settingsLoader.AllLlmProviders()

	cacheRows, err := s.llmRepo.GetAllModelCaches()
	if err != nil {
		cacheRows = nil
	}
	cacheMap := make(map[string][]dto.LlmModelDTO, len(cacheRows))
	for _, row := range cacheRows {
		var models []dto.LlmModelDTO
		if err := json.Unmarshal([]byte(row.ModelsJSON), &models); err == nil {
			cacheMap[row.ProviderAlias] = models
		}
	}

	providerDTOs := make([]dto.LlmProviderDTO, len(providers))
	for i, p := range providers {
		apiKey := p.ApiKey
		if len(apiKey) > 8 {
			apiKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
		}
		providerDTOs[i] = dto.LlmProviderDTO{
			ID:           p.ID,
			Alias:        p.Alias,
			Name:         p.Name,
			ApiUrl:       p.ApiUrl,
			ApiKey:       apiKey,
			Model:        p.Model,
			CachedModels: cacheMap[p.Alias],
		}
	}

	c.JSON(http.StatusOK, dto.LlmSettingsResponse{
		ID:                     settings.ID,
		ActiveProvider:         settings.ActiveProvider,
		VlProvider:             settings.VlProvider,
		TagScanEnabled:         settings.TagScanEnabled,
		TagScanStartHour:       settings.TagScanStartHour,
		TagScanStartMinute:     settings.TagScanStartMinute,
		TagScanEndHour:         settings.TagScanEndHour,
		TagScanEndMinute:       settings.TagScanEndMinute,
		TagScanTimezoneOffset:  settings.TagScanTimezoneOffset,
		EmbeddingProviderAlias: settings.EmbeddingProviderAlias,
		EmbeddingModel:         settings.EmbeddingModel,
		EmbeddingDimension:     settings.EmbeddingDimension,
		EmbeddingBatchSize:     settings.EmbeddingBatchSize,
		Providers:              providerDTOs,
	})
}

// handleUpdateLlmSettings updates LLM global settings (chat provider, VL provider, tag scan schedule)
func (s *Server) handleUpdateLlmSettings(c *gin.Context) {
	var req dto.UpdateLlmSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	settings, err := s.llmRepo.GetSettings()
	globalUpdates := make(map[string]interface{})
	if req.ActiveProvider != nil {
		globalUpdates["active_provider"] = *req.ActiveProvider
	}
	if req.VlProvider != nil {
		globalUpdates["vl_provider"] = *req.VlProvider
	}
	if req.TagScanEnabled != nil {
		globalUpdates["tag_scan_enabled"] = *req.TagScanEnabled
	}
	if req.TagScanStartHour != nil {
		globalUpdates["tag_scan_start_hour"] = *req.TagScanStartHour
	}
	if req.TagScanStartMinute != nil {
		globalUpdates["tag_scan_start_minute"] = *req.TagScanStartMinute
	}
	if req.TagScanEndHour != nil {
		globalUpdates["tag_scan_end_hour"] = *req.TagScanEndHour
	}
	if req.TagScanEndMinute != nil {
		globalUpdates["tag_scan_end_minute"] = *req.TagScanEndMinute
	}
	if req.TagScanTimezoneOffset != nil {
		globalUpdates["tag_scan_timezone_offset"] = *req.TagScanTimezoneOffset
	}
	if req.EmbeddingProviderAlias != nil {
		globalUpdates["embedding_provider_alias"] = *req.EmbeddingProviderAlias
	}
	if req.EmbeddingModel != nil {
		globalUpdates["embedding_model"] = *req.EmbeddingModel
	}
	if req.EmbeddingDimension != nil {
		globalUpdates["embedding_dimension"] = *req.EmbeddingDimension
	}
	if req.EmbeddingBatchSize != nil {
		globalUpdates["embedding_batch_size"] = *req.EmbeddingBatchSize
	}

	if err == gorm.ErrRecordNotFound {
		settings = &domain.LlmSettings{
			ActiveProvider:     "ollama_1",
			VlProvider:         "ollama_1",
			TagScanEnabled:     true,
			TagScanStartHour:   22,
			TagScanStartMinute: 0,
			TagScanEndHour:     7,
			TagScanEndMinute:   0,
		}
		if req.ActiveProvider != nil {
			settings.ActiveProvider = *req.ActiveProvider
		}
		if req.VlProvider != nil {
			settings.VlProvider = *req.VlProvider
		}
		if req.TagScanEnabled != nil {
			settings.TagScanEnabled = *req.TagScanEnabled
		}
		if req.TagScanStartHour != nil {
			settings.TagScanStartHour = *req.TagScanStartHour
		}
		if req.TagScanStartMinute != nil {
			settings.TagScanStartMinute = *req.TagScanStartMinute
		}
		if req.TagScanEndHour != nil {
			settings.TagScanEndHour = *req.TagScanEndHour
		}
		if req.TagScanEndMinute != nil {
			settings.TagScanEndMinute = *req.TagScanEndMinute
		}
		if req.TagScanTimezoneOffset != nil {
			settings.TagScanTimezoneOffset = *req.TagScanTimezoneOffset
		}
		if req.EmbeddingProviderAlias != nil {
			settings.EmbeddingProviderAlias = *req.EmbeddingProviderAlias
		}
		if req.EmbeddingModel != nil {
			settings.EmbeddingModel = *req.EmbeddingModel
		}
		if req.EmbeddingDimension != nil {
			settings.EmbeddingDimension = *req.EmbeddingDimension
		}
		if req.EmbeddingBatchSize != nil {
			settings.EmbeddingBatchSize = *req.EmbeddingBatchSize
		}
		if err := s.llmRepo.CreateSettings(settings); err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
		return
	} else {
		if len(globalUpdates) > 0 {
			s.llmRepo.UpdateSettings(globalUpdates)
		}
	}

	settings, _ = s.llmRepo.ReloadSettings()
	if settings != nil && s.tagScanManager != nil && s.tagScanManager.IsRunning() {
		s.tagScanManager.UpdateSchedule(settings.TagScanEnabled, settings.TagScanStartHour, settings.TagScanStartMinute, settings.TagScanEndHour, settings.TagScanEndMinute, settings.TagScanTimezoneOffset)
	}

	c.JSON(http.StatusOK, map[string]string{"message": string(i18n.MsgLlmOcrSettingsSaved)})
}

// handleProbeEmbeddingDimension probes the embedding model to detect its vector dimension.
func (s *Server) handleProbeEmbeddingDimension(c *gin.Context) {
	var req dto.ProbeEmbeddingDimensionRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	provider, err := s.llmRepo.GetProviderByAlias(req.ProviderAlias)
	if err != nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgEmbeddingProviderNotFound)
		return
	}

	embeddingClient, err := llm.NewEmbeddingClient(provider.Name, provider.ApiUrl, provider.ApiKey, req.Model)
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, i18n.MsgEmbeddingClientFailed)
		return
	}

	probe, err := embeddingClient.Embed(c.Request.Context(), []string{"dimension probe"})
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, i18n.MsgEmbeddingProbeFailed)
		return
	}
	if len(probe) == 0 || len(probe[0]) == 0 {
		s.respondError(c, http.StatusInternalServerError, i18n.MsgEmbeddingEmptyVector)
		return
	}

	dimension := len(probe[0])

	s.llmRepo.UpdateSettings(map[string]interface{}{
		"embedding_dimension":      dimension,
		"embedding_model":          req.Model,
		"embedding_provider_alias": req.ProviderAlias,
	})

	c.JSON(http.StatusOK, dto.ProbeEmbeddingDimensionResponse{Dimension: dimension})
}

// handleCreateLlmProvider creates a new LLM provider
func (s *Server) handleCreateLlmProvider(c *gin.Context) {
	var req dto.CreateLlmProviderRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if _, err := s.llmRepo.GetProviderByAlias(req.Alias); err == nil {
		c.JSON(http.StatusConflict, i18n.CreateValidationError(i18n.ValidationError))
		return
	}

	provider := domain.LlmProvider{
		Name:   req.Name,
		Alias:  req.Alias,
		ApiUrl: req.ApiUrl,
		ApiKey: req.ApiKey,
		Model:  req.Model,
	}
	if provider.ApiUrl == "" {
		switch provider.Name {
		case "ollama":
			provider.ApiUrl = "http://localhost:11434"
		case "ollama_cloud":
			provider.ApiUrl = "https://ollama.com"
		case "openai":
			provider.ApiUrl = "https://api.openai.com"
		}
	}
	if provider.Model == "" {
		provider.Model = "minicpm-v"
	}

	if err := s.llmRepo.CreateProvider(&provider); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
		return
	}

	c.JSON(http.StatusCreated, dto.LlmProviderDTO{
		ID:     provider.ID,
		Alias:  provider.Alias,
		Name:   provider.Name,
		ApiUrl: provider.ApiUrl,
		Model:  provider.Model,
	})
}

// handleUpdateLlmProvider updates an existing LLM provider by alias
func (s *Server) handleUpdateLlmProvider(c *gin.Context) {
	alias := c.Param("alias")
	var req dto.UpdateLlmProviderRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	if _, err := s.llmRepo.GetProviderByAlias(alias); err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return
	}

	updates := make(map[string]interface{})
	if req.ApiUrl != nil {
		updates["api_url"] = *req.ApiUrl
	}
	if req.ApiKey != nil {
		updates["api_key"] = *req.ApiKey
	}
	if req.Model != nil {
		updates["model"] = *req.Model
	}
	if req.Alias != nil && *req.Alias != alias {
		if _, err := s.llmRepo.GetProviderByAlias(*req.Alias); err == nil {
			c.JSON(http.StatusConflict, i18n.CreateValidationError(i18n.ValidationError))
			return
		}
		updates["alias"] = *req.Alias
		settings, err := s.llmRepo.GetSettings()
		if err == nil && settings.ActiveProvider == alias {
			s.llmRepo.UpdateSettings(map[string]interface{}{"active_provider": *req.Alias})
		}
	}

	if len(updates) > 0 {
		if err := s.llmRepo.UpdateProviderByAlias(alias, updates); err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
			return
		}

		if req.ApiUrl != nil || req.ApiKey != nil {
			s.llmRepo.DeleteModelCacheByAlias(alias)
		}

		if req.Alias != nil && *req.Alias != alias {
			s.llmRepo.UpdateModelCacheAlias(alias, *req.Alias)
		}
	}

	c.JSON(http.StatusOK, map[string]string{"message": string(i18n.MsgLlmOcrSettingsSaved)})
}

// handleDeleteLlmProvider deletes an LLM provider by alias
func (s *Server) handleDeleteLlmProvider(c *gin.Context) {
	alias := c.Param("alias")

	provider, err := s.llmRepo.GetProviderByAlias(alias)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsNotFound))
		return
	}

	settings, err := s.llmRepo.GetSettings()
	if err == nil && settings.ActiveProvider == alias {
		if firstProvider, err := s.llmRepo.GetFirstProviderExcept(provider.ID); err == nil {
			s.llmRepo.UpdateSettings(map[string]interface{}{"active_provider": firstProvider.Alias})
		}
	}

	if err := s.llmRepo.DeleteProvider(provider); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
		return
	}

	s.llmRepo.DeleteModelCacheByAlias(alias)

	c.JSON(http.StatusOK, map[string]string{"message": string(i18n.MsgLlmOcrSettingsSaved)})
}

// handleLlmRecognize starts LLM-based OCR recognition asynchronously
func (s *Server) handleLlmRecognize(c *gin.Context) {
	var req dto.LlmOcrRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	llmClient, provider, ok := s.llmFactory.CreateVLClient(c)
	if !ok {
		return
	}

	imageFile, err := s.imageFileRepo.FindByPath(req.ImagePath)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	if !req.Force && s.llmOcrService != nil {
		existing, _ := s.llmOcrService.GetRecognition(imageFile.ID)
		if existing != nil && existing.Success {
			c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{
				Status:           "completed",
				MarkdownContent:  existing.MarkdownContent,
				Language:         existing.Language,
				Provider:         existing.Provider,
				Model:            existing.Model,
				ProcessingTimeMs: existing.ProcessingTimeMs,
			})
			return
		}
	}

	if s.llmOcrService == nil {
		c.JSON(http.StatusServiceUnavailable, i18n.ErrorResponse(i18n.MsgLlmOcrRecognitionFailed))
		return
	}

	_ = s.llmOcrService.StartRecognizeAsync(imageFile.ID, llmClient, provider)
	c.JSON(http.StatusAccepted, dto.LlmRecognizeStatusResponse{
		Status: "processing",
	})
}

// handleLlmRecognizeStatus returns the status of an async LLM recognition task
func (s *Server) handleLlmRecognizeStatus(c *gin.Context) {
	imagePath := c.Query("path")
	if imagePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgOcrImagePathRequired))
		return
	}

	imageFile, err := s.imageFileRepo.FindByPath(imagePath)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	if s.llmOcrService == nil {
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "not_found"})
		return
	}

	taskStatus := s.llmOcrService.GetRecognizeStatus(imageFile.ID)
	if taskStatus == nil {
		existing, _ := s.llmOcrService.GetRecognition(imageFile.ID)
		if existing != nil && existing.Success {
			c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{
				Status:           "completed",
				MarkdownContent:  existing.MarkdownContent,
				Language:         existing.Language,
				Provider:         existing.Provider,
				Model:            existing.Model,
				ProcessingTimeMs: existing.ProcessingTimeMs,
			})
			return
		}
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "not_found"})
		return
	}

	switch taskStatus.Status {
	case "processing":
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "processing"})
	case "completed":
		resp := dto.LlmRecognizeStatusResponse{
			Status: "completed",
		}
		if taskStatus.Result != nil {
			resp.MarkdownContent = taskStatus.Result.MarkdownContent
			resp.Language = taskStatus.Result.Language
			resp.Provider = taskStatus.Result.Provider
			resp.Model = taskStatus.Result.Model
			resp.ProcessingTimeMs = taskStatus.Result.ProcessingTimeMs
		}
		c.JSON(http.StatusOK, resp)
	case "failed":
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{
			Status: "failed",
			Error:  taskStatus.Error,
		})
	default:
		c.JSON(http.StatusOK, dto.LlmRecognizeStatusResponse{Status: "not_found"})
	}
}

// handleGetLlmRecognition retrieves LLM OCR recognition for an image
func (s *Server) handleGetLlmRecognition(c *gin.Context) {
	imagePath := c.Query("path")
	if imagePath == "" {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgOcrImagePathRequired))
		return
	}

	imageFile, err := s.imageFileRepo.FindByPath(imagePath)
	if err != nil {
		c.JSON(http.StatusNotFound, i18n.ErrorResponse(i18n.MsgOcrDataNotFound))
		return
	}

	if s.llmOcrService == nil {
		c.JSON(http.StatusOK, dto.LlmOcrDataResponse{Found: false})
		return
	}

	recognition, err := s.llmOcrService.GetRecognition(imageFile.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrRecognitionFailed))
		return
	}

	if recognition == nil {
		c.JSON(http.StatusOK, dto.LlmOcrDataResponse{Found: false})
		return
	}

	c.JSON(http.StatusOK, dto.LlmOcrDataResponse{
		Found:            true,
		MarkdownContent:  recognition.MarkdownContent,
		Language:         recognition.Language,
		Provider:         recognition.Provider,
		Model:            recognition.Model,
		ProcessingTimeMs: recognition.ProcessingTimeMs,
		Success:          recognition.Success,
		Error:            recognition.Error,
		CreatedAt:        recognition.CreatedAt.Format(helpers.DateTimeFormat),
	})
}

// handleGetLlmModels returns a list of available LLM models from the configured server.
func (s *Server) handleGetLlmModels(c *gin.Context) {
	providerName := c.Query("provider")
	forceRefresh := c.Query("force") == "true"

	var provider domain.LlmProvider
	var found bool

	if providerName != "" {
		provider, found = s.settingsLoader.LlmProvider(providerName)
	} else {
		settings, settingsFound := s.settingsLoader.LlmSettingsIfExists()
		if !settingsFound {
			c.JSON(http.StatusNotFound, dto.LlmModelsResponse{
				Success:  false,
				Error:    "LLM settings not configured",
				Provider: "",
			})
			return
		}
		provider, found = s.settingsLoader.LlmProvider(settings.ActiveProvider)
	}

	if !found {
		c.JSON(http.StatusNotFound, dto.LlmModelsResponse{
			Success:  false,
			Error:    "Provider not configured",
			Provider: providerName,
		})
		return
	}

	if !forceRefresh {
		cacheRow, err := s.llmRepo.GetModelCacheByAlias(provider.Alias)
		if err == nil {
			var models []dto.LlmModelDTO
			if err := json.Unmarshal([]byte(cacheRow.ModelsJSON), &models); err == nil && len(models) > 0 {
				c.JSON(http.StatusOK, dto.LlmModelsResponse{
					Success:  true,
					Models:   models,
					Provider: provider.Name,
				})
				return
			}
		}
	}

	llmClient, err := llm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, provider.Model, s.config.LlmMaxImageMegapixels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.LlmModelsResponse{
			Success:  false,
			Error:    err.Error(),
			Provider: provider.Name,
		})
		return
	}

	models, err := llmClient.ListModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, dto.LlmModelsResponse{
			Success:  false,
			Error:    err.Error(),
			Provider: provider.Name,
		})
		return
	}

	modelDTOs := make([]dto.LlmModelDTO, len(models))
	for i, m := range models {
		modelDTOs[i] = dto.LlmModelDTO{
			ID:            m.ID,
			Name:          m.Name,
			Size:          m.Size,
			ContextLength: m.ContextLength,
			Capabilities:  m.Capabilities,
		}
	}

	if len(modelDTOs) > 0 {
		modelsJSON, _ := json.Marshal(modelDTOs)
		s.llmRepo.UpsertModelCache(&domain.LlmProviderModelCache{
			ProviderAlias: provider.Alias,
			ModelsJSON:    string(modelsJSON),
			FetchedAt:     time.Now(),
		})
	}

	c.JSON(http.StatusOK, dto.LlmModelsResponse{
		Success:  true,
		Models:   modelDTOs,
		Provider: provider.Name,
	})
}

// handleGetImageTags returns existing tags for an image (without generating new ones).
func (s *Server) handleGetImageTags(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path is required"})
		return
	}

	imageFile, err := s.imageFileRepo.FindByPath(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	existingTags, _ := s.imageTagRepo.FindByImageFileID(imageFile.ID)

	tags := make([]string, len(existingTags))
	for i, t := range existingTags {
		tags[i] = t.Tag
	}

	c.JSON(http.StatusOK, dto.ImageTagsResponse{Tags: tags})
}

// handleAiAction executes an AI action (describe, tags, recognizeText, askQuestion) asynchronously
func (s *Server) handleAiAction(c *gin.Context) {
	var req dto.AiActionRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	llmClient, provider, ok := s.llmFactory.CreateVLClient(c)
	if !ok {
		return
	}

	imageFile, err := s.imageFileRepo.FindByPath(req.ImagePath)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "Image not found",
		})
		return
	}

	if req.Action == dto.AiActionAskQuestion && req.Question == "" {
		c.JSON(http.StatusBadRequest, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "Question is required for askQuestion action",
		})
		return
	}

	if s.llmOcrService == nil {
		c.JSON(http.StatusServiceUnavailable, dto.AiActionResponse{
			Success: false,
			Action:  req.Action,
			Error:   "AI service not available",
		})
		return
	}

	taskID := uuid.New().String()

	if req.Action == dto.AiActionTags && !req.Force {
		existingTags, _ := s.imageTagRepo.FindByImageFileID(imageFile.ID)
		if len(existingTags) > 0 {
			tags := make([]string, len(existingTags))
			for i, t := range existingTags {
				tags[i] = t.Tag
			}
			s.llmOcrService.StoreCachedTagsResult(taskID, tags)

			c.JSON(http.StatusAccepted, dto.AiActionStartResponse{
				TaskID: taskID,
				Action: req.Action,
				Status: "processing",
			})
			return
		}
	}

	s.llmOcrService.StartAiActionAsync(taskID, imageFile.ID, string(req.Action), req.Question, req.Language, llmClient, provider)

	c.JSON(http.StatusAccepted, dto.AiActionStartResponse{
		TaskID: taskID,
		Action: req.Action,
		Status: "processing",
	})
}

// handleAiActionStatus returns the status of an async AI action task
func (s *Server) handleAiActionStatus(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, dto.AiActionStatusResponse{
			Status: "failed",
			Error:  "Task ID is required",
		})
		return
	}

	if s.llmOcrService == nil {
		c.JSON(http.StatusServiceUnavailable, dto.AiActionStatusResponse{
			Status: "failed",
			Error:  "AI service not available",
		})
		return
	}

	taskStatus := s.llmOcrService.GetAiActionStatus(taskID)
	if taskStatus == nil {
		c.JSON(http.StatusNotFound, dto.AiActionStatusResponse{
			Status: "failed",
			Error:  "Task not found or expired",
		})
		return
	}

	response := dto.AiActionStatusResponse{
		TaskID: taskID,
		Status: taskStatus.Status,
		Action: dto.AiActionType(taskStatus.Action),
	}

	if taskStatus.Status == "completed" && taskStatus.Result != nil {
		response.Provider = taskStatus.Result.Provider
		response.Model = taskStatus.Result.Model
		response.ProcessingTimeMs = taskStatus.Result.ProcessingTimeMs

		if taskStatus.Action == "tags" {
			response.Tags = taskStatus.Result.Tags
		} else {
			response.Result = taskStatus.Result.Result
		}
	} else if taskStatus.Status == "failed" {
		response.Error = taskStatus.Error
	}

	c.JSON(http.StatusOK, response)
}
