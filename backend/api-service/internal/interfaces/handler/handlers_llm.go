package handler

import (
	"net/http"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// instrumentTypeFromString converts a string to domain.InstrumentType.
func instrumentTypeFromString(s string) domain.InstrumentType {
	switch s {
	case "chat":
		return domain.InstrumentChat
	case "vl":
		return domain.InstrumentVL
	case "embedding":
		return domain.InstrumentEmbedding
	case "image_edit":
		return domain.InstrumentImageEdit
	default:
		return ""
	}
}

// buildInstrumentDTO builds a LlmInstrumentDTO from a domain LlmInstrumentSettings.
func buildInstrumentDTO(instr domain.LlmInstrumentSettings) dto.LlmInstrumentDTO {
	return dto.LlmInstrumentDTO{
		Type:          string(instr.Type),
		ProviderID:    instr.ProviderID,
		Model:         instr.Model,
		ProviderAlias: instr.Provider.Alias,
		ProviderName:  instr.Provider.Name,
	}
}

// buildTagScanDTO builds a TagScanSettingsDTO from a domain TagScanSettings.
func buildTagScanDTO(ts domain.TagScanSettings) dto.TagScanSettingsDTO {
	return dto.TagScanSettingsDTO{
		Enabled:        ts.Enabled,
		StartHour:      ts.StartHour,
		StartMinute:    ts.StartMinute,
		EndHour:        ts.EndHour,
		EndMinute:      ts.EndMinute,
		TimezoneOffset: ts.TimezoneOffset,
	}
}

// buildEmbeddingDTO builds an EmbeddingSettingsDTO from a domain EmbeddingSettings.
func buildEmbeddingDTO(es domain.EmbeddingSettings) dto.EmbeddingSettingsDTO {
	return dto.EmbeddingSettingsDTO{
		Dimension: es.Dimension,
		BatchSize: es.BatchSize,
	}
}

// domainModelsToDTOs converts domain.LlmProviderModel slices to dto.LlmModelDTO slices.
func domainModelsToDTOs(models []domain.LlmProviderModel) []dto.LlmModelDTO {
	dtos := make([]dto.LlmModelDTO, len(models))
	for i, m := range models {
		caps := make([]string, len(m.Capabilities))
		for j, c := range m.Capabilities {
			caps[j] = c.Capability
		}
		dtos[i] = dto.LlmModelDTO{
			ID:            m.ModelID,
			Name:          m.ModelName,
			Size:          m.Size,
			ContextLength: m.ContextLength,
			Capabilities:  caps,
		}
	}
	return dtos
}

// handleGetLlmSettings returns LLM settings with instruments, tag scan, embedding, and providers.
func (s *Server) handleGetLlmSettings(c *gin.Context) {
	providers := s.settingsLoader.AllLlmProviders()
	instruments := s.settingsLoader.AllLlmInstruments()
	tagScan := s.settingsLoader.TagScanSettings()
	embedding := s.settingsLoader.EmbeddingSettings()

	cacheMap := make(map[string][]dto.LlmModelDTO)
	for _, p := range providers {
		models, err := s.llmRepo.GetModelsByProviderID(p.ID)
		if err == nil && len(models) > 0 {
			cacheMap[p.Alias] = domainModelsToDTOs(models)
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
			CachedModels: cacheMap[p.Alias],
		}
	}

	instrumentDTOs := make([]dto.LlmInstrumentDTO, 0, len(instruments))
	for _, instr := range instruments {
		instrumentDTOs = append(instrumentDTOs, buildInstrumentDTO(instr))
	}

	c.JSON(http.StatusOK, dto.LlmSettingsResponse{
		Instruments: instrumentDTOs,
		TagScan:     buildTagScanDTO(tagScan),
		Embedding:   buildEmbeddingDTO(embedding),
		Providers:   providerDTOs,
	})
}

// handleUpdateLlmSettings updates LLM settings (instruments, tag scan, embedding).
func (s *Server) handleUpdateLlmSettings(c *gin.Context) {
	var req dto.UpdateLlmSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	// --- Handle instrument update (type + model + providerId) ---
	if req.InstrumentType != nil && *req.InstrumentType != "" {
		instType := instrumentTypeFromString(*req.InstrumentType)
		if instType == "" {
			c.JSON(http.StatusBadRequest, i18n.CreateValidationError(i18n.ValidationError))
			return
		}

		var instrument domain.LlmInstrumentSettings
		if err := s.db.Where("type = ?", instType).First(&instrument).Error; err != nil {
			// Create new instrument record
			instrument = domain.LlmInstrumentSettings{
				Type: instType,
			}
		}

		if req.ProviderID != nil {
			instrument.ProviderID = *req.ProviderID
		}
		if req.InstrumentModel != nil {
			instrument.Model = *req.InstrumentModel
		}

		if err := s.db.Save(&instrument).Error; err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
			return
		}
	}

	// --- Handle tag scan settings update ---
	tagScanDirty := req.TagScanEnabled != nil ||
		req.TagScanStartHour != nil ||
		req.TagScanStartMinute != nil ||
		req.TagScanEndHour != nil ||
		req.TagScanEndMinute != nil ||
		req.TagScanTimezoneOffset != nil

	if tagScanDirty {
		var ts domain.TagScanSettings
		if err := s.db.First(&ts).Error; err != nil {
			ts = domain.TagScanSettings{ID: 1}
		}
		if req.TagScanEnabled != nil {
			ts.Enabled = *req.TagScanEnabled
		}
		if req.TagScanStartHour != nil {
			ts.StartHour = *req.TagScanStartHour
		}
		if req.TagScanStartMinute != nil {
			ts.StartMinute = *req.TagScanStartMinute
		}
		if req.TagScanEndHour != nil {
			ts.EndHour = *req.TagScanEndHour
		}
		if req.TagScanEndMinute != nil {
			ts.EndMinute = *req.TagScanEndMinute
		}
		if req.TagScanTimezoneOffset != nil {
			ts.TimezoneOffset = *req.TagScanTimezoneOffset
		}
		if err := s.db.Save(&ts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
			return
		}

		// Update tag scan manager schedule
		if s.tagScanManager != nil && s.tagScanManager.IsRunning() {
			s.tagScanManager.UpdateSchedule(ts.Enabled, ts.StartHour, ts.StartMinute, ts.EndHour, ts.EndMinute, ts.TimezoneOffset)
		}
	}

	// --- Handle embedding settings update ---
	embDirty := req.EmbeddingDimension != nil || req.EmbeddingBatchSize != nil
	if embDirty {
		var es domain.EmbeddingSettings
		if err := s.db.First(&es).Error; err != nil {
			es = domain.EmbeddingSettings{ID: 1}
		}
		if req.EmbeddingDimension != nil {
			es.Dimension = *req.EmbeddingDimension
		}
		if req.EmbeddingBatchSize != nil {
			es.BatchSize = *req.EmbeddingBatchSize
		}
		if err := s.db.Save(&es).Error; err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
			return
		}
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

	// Store detected dimension in embedding_settings and update the embedding instrument
	var es domain.EmbeddingSettings
	if err := s.db.First(&es).Error; err != nil {
		es = domain.EmbeddingSettings{ID: 1}
	}
	es.Dimension = dimension
	s.db.Save(&es)

	// Also update the embedding instrument's model and provider
	var instr domain.LlmInstrumentSettings
	if err := s.db.Where("type = ?", domain.InstrumentEmbedding).First(&instr).Error; err == nil {
		instr.Model = req.Model
		instr.ProviderID = provider.ID
		s.db.Save(&instr)
	} else {
		s.db.Create(&domain.LlmInstrumentSettings{
			Type:       domain.InstrumentEmbedding,
			ProviderID: provider.ID,
			Model:      req.Model,
		})
	}

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

	if err := s.llmRepo.CreateProvider(&provider); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
		return
	}

	c.JSON(http.StatusCreated, dto.LlmProviderDTO{
		ID:     provider.ID,
		Alias:  provider.Alias,
		Name:   provider.Name,
		ApiUrl: provider.ApiUrl,
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
	if req.Alias != nil && *req.Alias != alias {
		if _, err := s.llmRepo.GetProviderByAlias(*req.Alias); err == nil {
			c.JSON(http.StatusConflict, i18n.CreateValidationError(i18n.ValidationError))
			return
		}
		updates["alias"] = *req.Alias
	}

	if len(updates) > 0 {
		if err := s.llmRepo.UpdateProviderByAlias(alias, updates); err != nil {
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
			return
		}

		if req.ApiUrl != nil || req.ApiKey != nil {
			if provider, err := s.llmRepo.GetProviderByAlias(alias); err == nil {
				s.llmRepo.DeleteModelsByProviderID(provider.ID)
			}
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

	// Remove any instrument settings referencing this provider (reassign to another provider)
	var otherProvider domain.LlmProvider
	if err := s.db.Where("id != ?", provider.ID).First(&otherProvider).Error; err == nil {
		s.db.Model(&domain.LlmInstrumentSettings{}).
			Where("provider_id = ?", provider.ID).
			Updates(map[string]interface{}{"provider_id": otherProvider.ID})
	}

	if err := s.llmRepo.DeleteProvider(provider); err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgLlmOcrSettingsSaveFailed))
		return
	}

	s.llmRepo.DeleteModelsByProviderID(provider.ID)

	c.JSON(http.StatusOK, map[string]string{"message": string(i18n.MsgLlmOcrSettingsSaved)})
}

// handleLlmRecognize starts LLM-based OCR recognition asynchronously
func (s *Server) handleLlmRecognize(c *gin.Context) {
	var req dto.LlmOcrRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	llmClient, provider, vlInstrument, ok := s.llmFactory.CreateVLClient(c)
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

	_ = s.llmOcrService.StartRecognizeAsync(imageFile.ID, llmClient, provider, vlInstrument.Model)
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
		// Default to chat instrument's provider
		instrument, instrFound := s.settingsLoader.LlmInstrumentByType(domain.InstrumentChat)
		if !instrFound {
			c.JSON(http.StatusNotFound, dto.LlmModelsResponse{
				Success:  false,
				Error:    "LLM settings not configured",
				Provider: "",
			})
			return
		}
		provider = instrument.Provider
		found = true
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
		dbModels, err := s.llmRepo.GetModelsByProviderID(provider.ID)
		if err == nil && len(dbModels) > 0 {
			modelDTOs := domainModelsToDTOs(dbModels)
			c.JSON(http.StatusOK, dto.LlmModelsResponse{
				Success:  true,
				Models:   modelDTOs,
				Provider: provider.Name,
			})
			return
		}
	}

	// Use the chat instrument's model for listing models (the provider itself no longer has a model)
	instrument, instrFound := s.settingsLoader.LlmInstrumentByType(domain.InstrumentChat)
	modelToUse := "minicpm-v"
	if instrFound {
		modelToUse = instrument.Model
	}

	llmClient, err := llm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, modelToUse, s.config.LlmMaxImageMegapixels)
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
	domainModels := make([]domain.LlmProviderModel, len(models))
	for i, m := range models {
		modelDTOs[i] = dto.LlmModelDTO{
			ID:            m.ID,
			Name:          m.Name,
			Size:          m.Size,
			ContextLength: m.ContextLength,
			Capabilities:  m.Capabilities,
		}
		domainModels[i] = domain.LlmProviderModel{
			ModelID:       m.ID,
			ModelName:     m.Name,
			Size:          m.Size,
			ContextLength: m.ContextLength,
		}
		if len(m.Capabilities) > 0 {
			domainModels[i].Capabilities = make([]domain.LlmModelCapability, len(m.Capabilities))
			for j, cap := range m.Capabilities {
				domainModels[i].Capabilities[j] = domain.LlmModelCapability{Capability: cap}
			}
		}
	}

	if len(domainModels) > 0 {
		s.llmRepo.ReplaceProviderModels(provider.ID, domainModels)
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

	llmClient, provider, vlInstrument, ok := s.llmFactory.CreateVLClient(c)
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

	s.llmOcrService.StartAiActionAsync(taskID, imageFile.ID, string(req.Action), req.Question, req.Language, llmClient, provider, vlInstrument.Model)

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
