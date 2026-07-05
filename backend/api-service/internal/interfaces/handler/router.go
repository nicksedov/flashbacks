package handler

import (
	"github.com/flashbacks/api-service/internal/interfaces/middleware"

	"github.com/gin-gonic/gin"
)

// SetupRouter sets up the Gin router with all API routes
func (s *Server) SetupRouter(authMiddleware *middleware.AuthMiddleware, csrfProtection *middleware.CSRFProtection, authHandlers *AuthHandlers) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Security headers middleware
	r.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
		c.Header("Pragma", "no-cache")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' ws: wss:; frame-ancestors 'none'")
		c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})

	// CORS middleware
	r.Use(middleware.SetupCORS(s.config))

	// CSRF protection
	r.Use(csrfProtection.Middleware())

	// Public auth routes
	api := r.Group("/api")
	{
		// Auth endpoints (public)
		auth := api.Group("/auth")
		{
			auth.GET("/status", authHandlers.handleAuthStatus)
			auth.POST("/login", authHandlers.handleLogin)
			auth.POST("/bootstrap/setup", authHandlers.handleBootstrapSetup)
		}

		// Protected routes (require auth)
		protected := api.Group("")
		protected.Use(authMiddleware.RequireAuth())
		protected.Use(middleware.LanguageMiddleware(s.db))
		{
			protected.POST("/auth/logout", authHandlers.handleLogout)
			protected.GET("/auth/me", authHandlers.handleMe)
			protected.POST("/auth/change-password", authHandlers.handleChangePassword)
			protected.PATCH("/users/me", authHandlers.handleUpdateProfile)
			protected.POST("/users/me/avatar", authHandlers.handleUploadAvatar)
			protected.DELETE("/users/me/avatar", authHandlers.handleDeleteAvatar)
			protected.GET("/users/:id/avatar", authHandlers.handleGetAvatar)

			// Thumbnail cache endpoints
			protected.GET("/thumbnail/cache/stats", s.handleThumbnailCacheStats)
			protected.DELETE("/thumbnail/cache/invalidate", s.handleThumbnailCacheInvalidate)
			protected.DELETE("/thumbnail/cache/invalidate-all", s.handleThumbnailCacheInvalidateAll)
			protected.POST("/thumbnail/cache/warmup", s.handleThumbnailCacheWarmup)
			protected.POST("/thumbnail/cache/enable", s.handleThumbnailCacheEnable)
			protected.POST("/thumbnail/cache/disable", s.handleThumbnailCacheDisable)

			// Existing endpoints (now protected)
			protected.GET("/duplicates", s.handleGetDuplicates)
			protected.POST("/scan", s.handleScan)
			protected.POST("/fast-scan", s.handleFastScan)
			protected.GET("/status", s.handleGetStatus)
			protected.POST("/delete-files", s.handleDeleteFiles)
			protected.POST("/move-files", s.handleMoveFiles)
			protected.GET("/thumbnail", s.handleThumbnail)
			protected.GET("/folder-patterns", s.handleGetFolderPatterns)
			protected.POST("/batch-delete", s.handleBatchDelete)
			protected.GET("/folders", s.handleGetFolders)
			protected.POST("/folders", s.handleAddFolder)
			protected.POST("/folders/mkdir", s.handleCreateSubfolder)
			protected.GET("/folders/subdirs", s.handleListSubdirs)
			protected.DELETE("/folders/:id", s.handleRemoveFolder)
			protected.GET("/gallery", s.handleGetGalleryImages)
			protected.GET("/gallery/calendar", s.handleGetGalleryCalendar)
			protected.GET("/gallery/calendar/month", s.handleGetCalendarMonthInfo)
			protected.GET("/gallery/calendar/dates", s.handleGetCalendarAllDates)
			protected.GET("/gallery/calendar/seek", s.handleGetCalendarSeek)
			protected.GET("/gallery/clusters", s.handleGetGalleryClusters)
			protected.GET("/gallery/geo-images", s.handleGetGeoImages)
			protected.GET("/image", s.handleServeImage)
			protected.GET("/ocr-image", s.handleServeOcrImage)
			protected.GET("/settings", s.handleGetSettings)
			protected.PUT("/settings", s.handleUpdateSettings)
			protected.GET("/sync-status", s.handleGetSyncStatus)
			protected.GET("/user-settings", s.handleGetUserSettings)
			protected.PUT("/user-settings", s.handleUpdateUserSettings)
			protected.GET("/trash-info", s.handleGetTrashInfo)
			protected.POST("/trash-clean", s.handleCleanTrash)
			protected.GET("/trash-list", s.handleListTrashFiles)
			protected.POST("/trash-restore", s.handleRestoreTrashFile)
			protected.POST("/trash-delete", s.handleDeleteTrashFile)
			protected.GET("/image-metadata", s.handleGetImageMetadata)
			protected.GET("/gallery/exif-images", s.handleGetImagesMissingExif)
			protected.GET("/geocode/search", s.handleGeocodeSearch)
			protected.PUT("/image-metadata/gps", s.handleUpdateGps)
			protected.PUT("/image-metadata/gps/batch", s.handleBatchUpdateGps)
			protected.GET("/image-metadata/location-candidates", s.handleGetLocationCandidates)
			protected.GET("/ocr-status", s.handleGetOCRStatus)
			protected.GET("/exif-status", s.handleGetExifStatus)
			protected.POST("/ocr/classify", s.handleStartOcrClassification)
			protected.POST("/ocr/classify-changes", s.handleStartOcrClassificationIncremental)
			protected.POST("/ocr/stop", s.handleStopOcrClassification)
			protected.GET("/ocr/classify-status", s.handleGetOcrClassificationStatus)
			protected.GET("/ocr/documents", s.handleGetOcrDocuments)
			protected.GET("/ocr/data", s.handleGetOcrData)

			// LLM OCR endpoints
			protected.GET("/llm/settings", s.handleGetLlmSettings)
			protected.PUT("/llm/settings", s.handleUpdateLlmSettings)
			protected.POST("/llm/providers", s.handleCreateLlmProvider)
			protected.PUT("/llm/providers/:alias", s.handleUpdateLlmProvider)
			protected.DELETE("/llm/providers/:alias", s.handleDeleteLlmProvider)
			protected.POST("/llm/recognize", s.handleLlmRecognize)
			protected.GET("/llm/recognize-status", s.handleLlmRecognizeStatus)
			protected.GET("/llm/recognition", s.handleGetLlmRecognition)
			protected.GET("/llm/models", s.handleGetLlmModels)
			protected.POST("/llm/embedding/probe", s.handleProbeEmbeddingDimension)

			// Image tags endpoint (read-only, no generation)
			protected.GET("/image-tags", s.handleGetImageTags)

			// AI Assistant endpoints
			protected.POST("/ai/action", s.handleAiAction)
			protected.GET("/ai/status/:taskId", s.handleAiActionStatus)

			// Smart Search (semantic search) endpoint
			protected.GET("/gallery/smart-search", s.handleSmartSearch)

			// Tag Scan endpoints
			protected.GET("/tag-scan/status", s.handleTagScanStatus)
			protected.POST("/tag-scan/pause", s.handleTagScanPause)
			protected.POST("/tag-scan/resume", s.handleTagScanResume)

			// Embedding backfill endpoints
			protected.GET("/embedding/status", s.handleEmbeddingStatus)
			protected.POST("/embedding/start", s.handleEmbeddingStart)
			protected.POST("/embedding/stop", s.handleEmbeddingStop)

			// Chat / Agent endpoints
			protected.POST("/chat/conversations", s.handleCreateConversation)
			protected.GET("/chat/conversations", s.handleListConversations)
			protected.DELETE("/chat/conversations/:id", s.handleDeleteConversation)
			protected.POST("/chat/conversations/:id/messages", s.handleSendMessage)
			protected.GET("/chat/conversations/:id/messages", s.handleGetMessages)

			// MCP endpoint
			if s.mcpServer != nil {
				protected.Any("/mcp", gin.WrapH(s.mcpServer.HTTPHandler()))
			}

			// Admin routes
			admin := protected.Group("/admin")
			admin.Use(middleware.RequireAdmin())
			{
				admin.GET("/users", authHandlers.handleListUsers)
				admin.POST("/users", authHandlers.handleCreateUser)
				admin.PATCH("/users/:id", authHandlers.handleUpdateUser)
				admin.DELETE("/users/:id", authHandlers.handleDeleteUser)
				admin.POST("/users/:id/reset-password", authHandlers.handleResetPassword)
				admin.GET("/audit", authHandlers.handleAuditLogs)
			}
		}
	}

	return r
}
