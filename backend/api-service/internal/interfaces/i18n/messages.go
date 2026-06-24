package i18n

// MessageKey represents a unique key for i18n messages
type MessageKey string

const (
	// General messages
	Success         MessageKey = "success"
	Error           MessageKey = "error"
	ValidationError MessageKey = "validation_error"

	// Authentication messages
	MsgAuthInternalError          MessageKey = "auth.internal_error"
	MsgAuthInvalidCredentials     MessageKey = "auth.invalid_credentials"
	MsgAuthRateLimited            MessageKey = "auth.rate_limited"
	MsgAuthBootstrapMode          MessageKey = "auth.bootstrap_mode"
	MsgAuthLogoutSuccess          MessageKey = "auth.logout_success"
	MsgAuthUnauthorized           MessageKey = "auth.unauthorized"
	MsgAuthInvalidRequestFormat   MessageKey = "auth.invalid_request_format"
	MsgAuthPasswordLength         MessageKey = "auth.password_length"
	MsgAuthInvalidCurrentPassword MessageKey = "auth.invalid_current_password"
	MsgAuthPasswordChangeFailed   MessageKey = "auth.password_change_failed"
	MsgAuthBootstrapFailed        MessageKey = "auth.bootstrap_failed"
	MsgAuthSessionCreationFailed  MessageKey = "auth.session_creation_failed"
	MsgAuthBootstrapComplete      MessageKey = "auth.bootstrap_complete"
	MsgAuthUsersListFailed        MessageKey = "auth.users_list_failed"
	MsgAuthInvalidRole            MessageKey = "auth.invalid_role"
	MsgAuthUserCreated            MessageKey = "auth.user_created"
	MsgAuthUserNotFound           MessageKey = "auth.user_not_found"
	MsgAuthUserUpdated            MessageKey = "auth.user_updated"
	MsgAuthUserUpdateFailed       MessageKey = "auth.user_update_failed"
	MsgAuthUserDeleted            MessageKey = "auth.user_deleted"
	MsgAuthUserDeleteFailed       MessageKey = "auth.user_delete_failed"
	MsgAuthPasswordResetFailed    MessageKey = "auth.password_reset_failed"
	MsgAuthPasswordResetSuccess   MessageKey = "auth.password_reset_success"
	MsgAuthProfileUpdateFailed    MessageKey = "auth.profile_update_failed"
	MsgAuthAuditLogsFailed        MessageKey = "auth.audit_logs_failed"

	// Avatar messages
	MsgAvatarUploadFailed MessageKey = "avatar.upload_failed"
	MsgAvatarInvalidType  MessageKey = "avatar.invalid_type"
	MsgAvatarTooLarge     MessageKey = "avatar.too_large"
	MsgAvatarDeleteFailed MessageKey = "avatar.delete_failed"
	MsgAvatarNotFound     MessageKey = "avatar.not_found"

	// Scan messages
	MsgScanStarted         MessageKey = "scan.started"
	MsgScanFailed          MessageKey = "scan.failed"
	MsgScanDuplicateFailed MessageKey = "scan.duplicate_failed"
	MsgScanNoFilesSelected MessageKey = "scan.no_files_selected"
	MsgScanTrashDirFailed  MessageKey = "scan.trash_dir_failed"

	// Folder messages
	MsgFolderPathRequired     MessageKey = "folder.path_required"
	MsgFolderInvalidPath      MessageKey = "folder.invalid_path"
	MsgFolderCannotAccessPath MessageKey = "folder.cannot_access_path"
	MsgFolderNotDirectory     MessageKey = "folder.not_directory"
	MsgFolderConflictTrash    MessageKey = "folder.conflict_trash"
	MsgFolderAlreadyInGallery MessageKey = "folder.already_in_gallery"
	MsgFolderAddFailed        MessageKey = "folder.add_failed"
	MsgFolderAdded            MessageKey = "folder.added"
	MsgFolderNotFound         MessageKey = "folder.not_found"
	MsgFolderRemoved          MessageKey = "folder.removed"
	MsgFolderRemoveFailed     MessageKey = "folder.remove_failed"

	// Image messages
	MsgImagePathRequired       MessageKey = "image.path_required"
	MsgImageAccessDenied       MessageKey = "image.access_denied"
	MsgImageNotFound           MessageKey = "image.not_found"
	MsgImageInvalidTheme       MessageKey = "image.invalid_theme"
	MsgImageInvalidLanguage    MessageKey = "image.invalid_language"
	MsgImageInvalidTrashPath   MessageKey = "image.invalid_trash_path"
	MsgImageTrashConflict      MessageKey = "image.trash_conflict"
	MsgImageInvalidBackupPath  MessageKey = "image.invalid_backup_path"
	MsgImageBackupConflict     MessageKey = "image.backup_conflict"
	MsgImageTrashNotConfigured MessageKey = "image.trash_not_configured"
	MsgImageTrashNotExists     MessageKey = "image.trash_not_exists"
	MsgImageTrashReadFailed    MessageKey = "image.trash_read_failed"
	MsgImageTrashCleanFailed   MessageKey = "image.trash_clean_failed"
	MsgImageThumbnailFailed    MessageKey = "image.thumbnail_failed"
	MsgImageMetadataFailed     MessageKey = "image.metadata_failed"

	// User service messages
	MsgUserServiceInvalidRole         MessageKey = "user_service.invalid_role"
	MsgUserServicePasswordLength      MessageKey = "user_service.password_length"
	MsgUserServiceUserExists          MessageKey = "user_service.user_exists"
	MsgUserServiceLastAdminDemote     MessageKey = "user_service.last_admin_demote"
	MsgUserServiceLastAdminDeactivate MessageKey = "user_service.last_admin_deactivate"
	MsgUserServiceLastAdminDelete     MessageKey = "user_service.last_admin_delete"

	// Middleware messages
	MsgMiddlewareUnauthorized MessageKey = "middleware.unauthorized"
	MsgMiddlewareForbidden    MessageKey = "middleware.forbidden"
	MsgMiddlewareCSRFFailed   MessageKey = "middleware.csrf_failed"

	// Trash messages
	MsgTrashNotConfigured MessageKey = "trash.not_configured"
	MsgTrashNotExists     MessageKey = "trash.not_exists"
	MsgTrashReadFailed    MessageKey = "trash.read_failed"

	// Gallery messages
	MsgGalleryConflict MessageKey = "gallery.conflict"

	// OCR messages
	MsgOcrStarted           MessageKey = "ocr.started"
	MsgOcrFailed            MessageKey = "ocr.failed"
	MsgOcrAlreadyRunning    MessageKey = "ocr.already_running"
	MsgOcrNotRunning        MessageKey = "ocr.not_running"
	MsgOcrImagePathRequired MessageKey = "ocr.image_path_required"
	MsgOcrDataNotFound      MessageKey = "ocr.data_not_found"

	// LLM OCR messages
	MsgLlmOcrNotEnabled         MessageKey = "llm_ocr.not_enabled"
	MsgLlmOcrSettingsNotFound   MessageKey = "llm_ocr.settings_not_found"
	MsgLlmOcrRecognitionFailed  MessageKey = "llm_ocr.recognition_failed"
	MsgLlmOcrRecognitionStarted MessageKey = "llm_ocr.recognition_started"
	MsgLlmOcrSettingsSaved      MessageKey = "llm_ocr.settings_saved"
	MsgLlmOcrSettingsSaveFailed MessageKey = "llm_ocr.settings_save_failed"
	MsgLlmOcrNoRecognition      MessageKey = "llm_ocr.no_recognition"

	// Thumbnail cache messages
	MsgThumbnailCacheNotAvailable   MessageKey = "thumbnail_cache.not_available"
	MsgThumbnailCacheInvalidated    MessageKey = "thumbnail_cache.invalidated"
	MsgThumbnailCacheAllInvalidated MessageKey = "thumbnail_cache.all_invalidated"
	MsgThumbnailCacheWarmedUp       MessageKey = "thumbnail_cache.warmed_up"
	MsgThumbnailCacheEnabled        MessageKey = "thumbnail_cache.enabled"
	MsgThumbnailCacheDisabled       MessageKey = "thumbnail_cache.disabled"

	// Calendar messages
	MsgCalendarMonthYearRequired MessageKey = "calendar.month_year_required"
	MsgCalendarInvalidMonthYear  MessageKey = "calendar.invalid_month_year"
	MsgCalendarInvalidCursor     MessageKey = "calendar.invalid_cursor"

	// Geo messages
	MsgGeoInvalidZoom       MessageKey = "geo.invalid_zoom"
	MsgGeoInvalidDimensions MessageKey = "geo.invalid_dimensions"
	MsgGeoClusterFailed     MessageKey = "geo.cluster_failed"
	MsgGeoClusterNotFound   MessageKey = "geo.cluster_not_found"

	// Trash restore messages
	MsgTrashFileNameRequired MessageKey = "trash.file_name_required"
	MsgTrashFileNotFound     MessageKey = "trash.file_not_found"
	MsgTrashDeleteFailed     MessageKey = "trash.delete_failed"
	MsgTrashRestoreFailed    MessageKey = "trash.restore_failed"
	MsgTrashRestored         MessageKey = "trash.restored"
	MsgTrashFileDeleted      MessageKey = "trash.file_deleted"

	// LLM messages
	MsgLlmModelsFailed MessageKey = "llm.models_failed"

	// Geocode / GPS messages
	MsgGeocodeQueryRequired    MessageKey = "geocode.query_required"
	MsgGeocodeDateRequired     MessageKey = "geocode.date_required"
	MsgGeocodeSearchFailed    MessageKey = "geocode.search_failed"
	MsgGpsUpdateFailed       MessageKey = "geocode.gps_update_failed"
	MsgGpsInvalidCoordinates MessageKey = "geocode.invalid_coordinates"
	MsgGpsUpdated            MessageKey = "geocode.gps_updated"
	MsgGpsNotJpeg            MessageKey = "geocode.not_jpeg"
	MsgGpsBackupFailed       MessageKey = "geocode.backup_failed"
	MsgBatchGpsNoPaths       MessageKey = "batch_gps.no_paths"

	// Smart search messages
	MsgSmartSearchFailed  MessageKey = "smart.search_failed"
	MsgSmartQueryRequired MessageKey = "smart.query_required"

	// Chat messages
	MsgChatInvalidRequest        MessageKey = "chat.invalid_request"
	MsgChatConversationNotFound  MessageKey = "chat.conversation_not_found"
	MsgChatInvalidConversationID MessageKey = "chat.invalid_conversation_id"
	MsgChatContentRequired       MessageKey = "chat.content_required"
	MsgChatLlmNoChatSupport      MessageKey = "chat.llm_no_chat_support"
	MsgChatConversationDeleted   MessageKey = "chat.conversation_deleted"

	// Embedding messages
	MsgEmbeddingManagerNotAvailable MessageKey = "embedding.manager_not_available"
	MsgEmbeddingBackfillStarted    MessageKey = "embedding.backfill_started"
	MsgEmbeddingBackfillStopped    MessageKey = "embedding.backfill_stopped"
	MsgEmbeddingProviderNotFound   MessageKey = "embedding.provider_not_found"
	MsgEmbeddingClientFailed       MessageKey = "embedding.client_creation_failed"
	MsgEmbeddingProbeFailed        MessageKey = "embedding.probe_failed"
	MsgEmbeddingEmptyVector        MessageKey = "embedding.empty_vector"

	// Tag scan messages
	MsgTagScanManagerNotAvailable MessageKey = "tag_scan.manager_not_available"
	MsgTagScanPaused             MessageKey = "tag_scan.paused"
	MsgTagScanResumed            MessageKey = "tag_scan.resumed"
)

// GetMessage returns the message key as string
// This is used for JSON response serialization
func (k MessageKey) GetMessage() string {
	return string(k)
}

// ResponseMessage wraps a message key for JSON responses
type ResponseMessage struct {
	Message MessageKey `json:"message"`
}

// ResponseError wraps an error message key for JSON error responses
type ResponseError struct {
	Error MessageKey `json:"error"`
}

// SuccessResponse creates a success response with an optional message
func SuccessResponse(msg MessageKey, data ...interface{}) (map[string]interface{}, MessageKey) {
	resp := map[string]interface{}{"message": msg}
	if len(data) > 0 {
		resp["data"] = data[0]
	}
	return resp, msg
}

// ErrorResponse creates an error response
func ErrorResponse(msg MessageKey) map[string]interface{} {
	return map[string]interface{}{"error": msg}
}

// CreateValidationError creates a validation error response
func CreateValidationError(msg MessageKey) map[string]interface{} {
	return map[string]interface{}{"error": msg, "type": "validation"}
}

// ResolveMessage translates a message key to human-readable text using the i18n service
// This function should be called when sending responses to the client
func ResolveMessage(svc *Service, msg MessageKey, lang string) string {
	if svc == nil {
		return string(msg)
	}
	return svc.GetMessage(msg, lang)
}

// SuccessResponseResolved creates a success response with a resolved (translated) message
func SuccessResponseResolved(svc *Service, msg MessageKey, lang string, data ...interface{}) map[string]interface{} {
	resp := map[string]interface{}{"message": svc.GetMessage(msg, lang)}
	if len(data) > 0 {
		resp["data"] = data[0]
	}
	return resp
}

// ErrorResponseResolved creates an error response with a resolved (translated) message
func ErrorResponseResolved(svc *Service, msg MessageKey, lang string) map[string]interface{} {
	return map[string]interface{}{"error": svc.GetMessage(msg, lang)}
}

// ValidationErrorResolved creates a validation error response with a resolved message
func ValidationErrorResolved(svc *Service, msg MessageKey, lang string) map[string]interface{} {
	return map[string]interface{}{"error": svc.GetMessage(msg, lang), "type": "validation"}
}
