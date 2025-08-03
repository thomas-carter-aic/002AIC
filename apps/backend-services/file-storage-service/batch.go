package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Batch operations and storage management

// Batch upload files
func (s *FileStorageService) batchUpload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files provided"})
		return
	}

	userID := c.PostForm("user_id")
	projectID := c.PostForm("project_id")
	storageType := c.DefaultPostForm("storage_type", StorageTypeMinio)

	var results []gin.H
	var successCount, failureCount int

	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			results = append(results, gin.H{
				"index":    i,
				"filename": fileHeader.Filename,
				"status":   "failed",
				"error":    "Failed to open file",
			})
			failureCount++
			continue
		}

		// Check file size
		if fileHeader.Size > s.config.MaxFileSize {
			file.Close()
			results = append(results, gin.H{
				"index":    i,
				"filename": fileHeader.Filename,
				"status":   "failed",
				"error":    "File too large",
			})
			failureCount++
			continue
		}

		// Calculate hashes
		md5Hash, sha256Hash, err := calculateHashes(file)
		if err != nil {
			file.Close()
			results = append(results, gin.H{
				"index":    i,
				"filename": fileHeader.Filename,
				"status":   "failed",
				"error":    "Failed to calculate hashes",
			})
			failureCount++
			continue
		}

		// Check for duplicates
		var existingFile FileMetadata
		if err := s.db.Where("md5_hash = ? AND status = ?", md5Hash, FileStatusActive).First(&existingFile).Error; err == nil {
			file.Close()
			results = append(results, gin.H{
				"index":     i,
				"filename":  fileHeader.Filename,
				"status":    "duplicate",
				"file_id":   existingFile.ID,
				"message":   "File already exists",
			})
			continue
		}

		// Create file metadata
		fileID := uuid.New().String()
		extension := filepath.Ext(fileHeader.Filename)
		storedName := fmt.Sprintf("%s%s", fileID, extension)

		metadata := &FileMetadata{
			ID:           fileID,
			OriginalName: fileHeader.Filename,
			StoredName:   storedName,
			Size:         fileHeader.Size,
			MimeType:     fileHeader.Header.Get("Content-Type"),
			Extension:    extension,
			MD5Hash:      md5Hash,
			SHA256Hash:   sha256Hash,
			StorageType:  storageType,
			Status:       FileStatusUploading,
			Version:      1,
			UserID:       userID,
			ProjectID:    projectID,
			Metadata:     make(map[string]string),
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}

		// Store file
		var storagePath string
		switch storageType {
		case StorageTypeMinio:
			storagePath, err = s.storeFileInMinio(file, storedName, fileHeader.Size)
		case StorageTypeLocal:
			storagePath, err = s.storeFileLocally(file, storedName)
		default:
			err = fmt.Errorf("unsupported storage type: %s", storageType)
		}

		file.Close()

		if err != nil {
			results = append(results, gin.H{
				"index":    i,
				"filename": fileHeader.Filename,
				"status":   "failed",
				"error":    "Failed to store file",
			})
			failureCount++
			continue
		}

		metadata.Path = storagePath
		metadata.StorageLocation = storagePath
		metadata.Status = FileStatusActive

		// Save to database
		if err := s.db.Create(metadata).Error; err != nil {
			s.cleanupStoredFile(storageType, storagePath)
			results = append(results, gin.H{
				"index":    i,
				"filename": fileHeader.Filename,
				"status":   "failed",
				"error":    "Failed to save metadata",
			})
			failureCount++
			continue
		}

		// Update metrics
		sizeCategory := getSizeCategory(fileHeader.Size)
		filesUploaded.WithLabelValues(storageType, metadata.MimeType).Inc()
		storageUsed.WithLabelValues(storageType, userID).Add(float64(fileHeader.Size))

		results = append(results, gin.H{
			"index":     i,
			"filename":  fileHeader.Filename,
			"status":    "success",
			"file_id":   fileID,
			"size":      fileHeader.Size,
		})
		successCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"total_files":    len(files),
		"success_count":  successCount,
		"failure_count":  failureCount,
		"results":        results,
	})
}

// Batch delete files
func (s *FileStorageService) batchDelete(c *gin.Context) {
	var req BatchOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	permanent := c.Query("permanent") == "true"
	var results []gin.H
	var successCount, failureCount int

	for _, fileID := range req.FileIDs {
		var metadata FileMetadata
		if err := s.db.First(&metadata, "id = ? AND status != ?", fileID, FileStatusDeleted).Error; err != nil {
			results = append(results, gin.H{
				"file_id": fileID,
				"status":  "failed",
				"error":   "File not found",
			})
			failureCount++
			continue
		}

		if permanent {
			// Permanently delete file
			if err := s.deleteStoredFile(metadata.StorageType, metadata.Path); err != nil {
				results = append(results, gin.H{
					"file_id": fileID,
					"status":  "failed",
					"error":   "Failed to delete stored file",
				})
				failureCount++
				continue
			}

			// Delete from database
			if err := s.db.Delete(&metadata).Error; err != nil {
				results = append(results, gin.H{
					"file_id": fileID,
					"status":  "failed",
					"error":   "Failed to delete metadata",
				})
				failureCount++
				continue
			}

			// Update storage metrics
			storageUsed.WithLabelValues(metadata.StorageType, metadata.UserID).Sub(float64(metadata.Size))
		} else {
			// Soft delete
			metadata.Status = FileStatusDeleted
			metadata.UpdatedAt = time.Now().UTC()

			if err := s.db.Save(&metadata).Error; err != nil {
				results = append(results, gin.H{
					"file_id": fileID,
					"status":  "failed",
					"error":   "Failed to mark file as deleted",
				})
				failureCount++
				continue
			}
		}

		// Remove from cache
		s.removeCachedFileMetadata(fileID)

		results = append(results, gin.H{
			"file_id":   fileID,
			"status":    "success",
			"permanent": permanent,
		})
		successCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"total_files":   len(req.FileIDs),
		"success_count": successCount,
		"failure_count": failureCount,
		"permanent":     permanent,
		"results":       results,
	})
}

// Batch move files
func (s *FileStorageService) batchMove(c *gin.Context) {
	var req BatchMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var results []gin.H
	var successCount, failureCount int

	for _, fileID := range req.FileIDs {
		var metadata FileMetadata
		if err := s.db.First(&metadata, "id = ? AND status = ?", fileID, FileStatusActive).Error; err != nil {
			results = append(results, gin.H{
				"file_id": fileID,
				"status":  "failed",
				"error":   "File not found",
			})
			failureCount++
			continue
		}

		// Update metadata
		if req.Destination != "" {
			metadata.ProjectID = req.Destination
		}
		if req.Metadata != nil {
			for key, value := range req.Metadata {
				metadata.Metadata[key] = value
			}
		}
		metadata.UpdatedAt = time.Now().UTC()

		if err := s.db.Save(&metadata).Error; err != nil {
			results = append(results, gin.H{
				"file_id": fileID,
				"status":  "failed",
				"error":   "Failed to update metadata",
			})
			failureCount++
			continue
		}

		// Update cache
		s.cacheFileMetadata(&metadata)

		results = append(results, gin.H{
			"file_id":     fileID,
			"status":      "success",
			"destination": req.Destination,
		})
		successCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"total_files":   len(req.FileIDs),
		"success_count": successCount,
		"failure_count": failureCount,
		"destination":   req.Destination,
		"results":       results,
	})
}

// Get storage statistics
func (s *FileStorageService) getStorageStats(c *gin.Context) {
	userID := c.Query("user_id")
	projectID := c.Query("project_id")

	stats := gin.H{
		"timestamp": time.Now().UTC(),
	}

	// Base query
	query := s.db.Model(&FileMetadata{}).Where("status = ?", FileStatusActive)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}

	// Total files and size
	var totalFiles int64
	var totalSize int64
	query.Count(&totalFiles)
	query.Select("COALESCE(SUM(size), 0)").Scan(&totalSize)

	stats["total_files"] = totalFiles
	stats["total_size"] = totalSize
	stats["total_size_human"] = formatBytes(totalSize)

	// Files by storage type
	var storageStats []struct {
		StorageType string `json:"storage_type"`
		Count       int64  `json:"count"`
		Size        int64  `json:"size"`
	}
	query.Select("storage_type, COUNT(*) as count, COALESCE(SUM(size), 0) as size").
		Group("storage_type").
		Find(&storageStats)
	stats["by_storage_type"] = storageStats

	// Files by mime type
	var mimeStats []struct {
		MimeType string `json:"mime_type"`
		Count    int64  `json:"count"`
		Size     int64  `json:"size"`
	}
	query.Select("mime_type, COUNT(*) as count, COALESCE(SUM(size), 0) as size").
		Group("mime_type").
		Order("count DESC").
		Limit(10).
		Find(&mimeStats)
	stats["by_mime_type"] = mimeStats

	// Files by date (last 30 days)
	var dateStats []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
		Size  int64  `json:"size"`
	}
	query.Select("DATE(created_at) as date, COUNT(*) as count, COALESCE(SUM(size), 0) as size").
		Where("created_at >= ?", time.Now().AddDate(0, 0, -30)).
		Group("DATE(created_at)").
		Order("date DESC").
		Find(&dateStats)
	stats["by_date"] = dateStats

	// Top users by storage usage (if not filtered by user)
	if userID == "" {
		var userStats []struct {
			UserID string `json:"user_id"`
			Count  int64  `json:"count"`
			Size   int64  `json:"size"`
		}
		query.Select("user_id, COUNT(*) as count, COALESCE(SUM(size), 0) as size").
			Where("user_id != ''").
			Group("user_id").
			Order("size DESC").
			Limit(10).
			Find(&userStats)
		stats["top_users"] = userStats
	}

	c.JSON(http.StatusOK, stats)
}

// Cleanup storage
func (s *FileStorageService) cleanupStorage(c *gin.Context) {
	dryRun := c.Query("dry_run") == "true"
	olderThan := c.DefaultQuery("older_than", "30d")

	// Parse duration
	var cutoffDate time.Time
	switch olderThan {
	case "1d":
		cutoffDate = time.Now().AddDate(0, 0, -1)
	case "7d":
		cutoffDate = time.Now().AddDate(0, 0, -7)
	case "30d":
		cutoffDate = time.Now().AddDate(0, 0, -30)
	case "90d":
		cutoffDate = time.Now().AddDate(0, 0, -90)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid older_than parameter"})
		return
	}

	// Find files to cleanup
	var filesToCleanup []FileMetadata
	if err := s.db.Where("status = ? AND updated_at < ?", FileStatusDeleted, cutoffDate).
		Find(&filesToCleanup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find files to cleanup"})
		return
	}

	var cleanedCount int
	var freedSpace int64
	var errors []string

	for _, file := range filesToCleanup {
		if !dryRun {
			// Delete actual file
			if err := s.deleteStoredFile(file.StorageType, file.Path); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to delete file %s: %v", file.ID, err))
				continue
			}

			// Delete from database
			if err := s.db.Delete(&file).Error; err != nil {
				errors = append(errors, fmt.Sprintf("Failed to delete metadata for file %s: %v", file.ID, err))
				continue
			}

			// Remove from cache
			s.removeCachedFileMetadata(file.ID)
		}

		cleanedCount++
		freedSpace += file.Size
	}

	result := gin.H{
		"dry_run":       dryRun,
		"older_than":    olderThan,
		"cutoff_date":   cutoffDate,
		"files_found":   len(filesToCleanup),
		"files_cleaned": cleanedCount,
		"space_freed":   freedSpace,
		"space_freed_human": formatBytes(freedSpace),
	}

	if len(errors) > 0 {
		result["errors"] = errors
	}

	if dryRun {
		result["message"] = "Dry run completed - no files were actually deleted"
	} else {
		result["message"] = "Cleanup completed successfully"
	}

	c.JSON(http.StatusOK, result)
}

// Migrate storage
func (s *FileStorageService) migrateStorage(c *gin.Context) {
	var req struct {
		FromStorage string   `json:"from_storage" binding:"required"`
		ToStorage   string   `json:"to_storage" binding:"required"`
		FileIDs     []string `json:"file_ids"`
		UserID      string   `json:"user_id"`
		ProjectID   string   `json:"project_id"`
		DryRun      bool     `json:"dry_run"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate storage types
	validStorageTypes := []string{StorageTypeLocal, StorageTypeMinio, StorageTypeS3}
	isValidStorage := func(storageType string) bool {
		for _, valid := range validStorageTypes {
			if storageType == valid {
				return true
			}
		}
		return false
	}

	if !isValidStorage(req.FromStorage) || !isValidStorage(req.ToStorage) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid storage type"})
		return
	}

	// Build query to find files to migrate
	query := s.db.Model(&FileMetadata{}).Where("storage_type = ? AND status = ?", req.FromStorage, FileStatusActive)

	if len(req.FileIDs) > 0 {
		query = query.Where("id IN ?", req.FileIDs)
	}
	if req.UserID != "" {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.ProjectID != "" {
		query = query.Where("project_id = ?", req.ProjectID)
	}

	var filesToMigrate []FileMetadata
	if err := query.Find(&filesToMigrate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find files to migrate"})
		return
	}

	var migratedCount int
	var migratedSize int64
	var errors []string

	for _, file := range filesToMigrate {
		if req.DryRun {
			migratedCount++
			migratedSize += file.Size
			continue
		}

		// Read file from source storage
		var fileData io.Reader
		var err error

		switch req.FromStorage {
		case StorageTypeLocal:
			var localFile *os.File
			localFile, err = os.Open(file.Path)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to open local file %s: %v", file.ID, err))
				continue
			}
			fileData = localFile
			defer localFile.Close()

		case StorageTypeMinio:
			ctx := context.Background()
			fileData, err = s.minioClient.GetObject(ctx, s.config.MinioBucket, file.StoredName, minio.GetObjectOptions{})
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to get MinIO file %s: %v", file.ID, err))
				continue
			}
		}

		// Store file in destination storage
		var newPath string
		switch req.ToStorage {
		case StorageTypeLocal:
			// Create a temporary multipart file wrapper
			// This is simplified - in production you'd need a proper implementation
			errors = append(errors, fmt.Sprintf("Local storage migration not fully implemented for file %s", file.ID))
			continue

		case StorageTypeMinio:
			ctx := context.Background()
			_, err = s.minioClient.PutObject(ctx, s.config.MinioBucket, file.StoredName, fileData, file.Size, minio.PutObjectOptions{
				ContentType: file.MimeType,
			})
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to store file %s in MinIO: %v", file.ID, err))
				continue
			}
			newPath = fmt.Sprintf("minio://%s/%s", s.config.MinioBucket, file.StoredName)
		}

		// Update file metadata
		oldPath := file.Path
		file.StorageType = req.ToStorage
		file.Path = newPath
		file.StorageLocation = newPath
		file.UpdatedAt = time.Now().UTC()

		if err := s.db.Save(&file).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Failed to update metadata for file %s: %v", file.ID, err))
			continue
		}

		// Delete from source storage
		if err := s.deleteStoredFile(req.FromStorage, oldPath); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to delete source file %s: %v", file.ID, err))
			// Don't continue here as the file was successfully migrated
		}

		// Update cache
		s.cacheFileMetadata(&file)

		migratedCount++
		migratedSize += file.Size
	}

	result := gin.H{
		"dry_run":         req.DryRun,
		"from_storage":    req.FromStorage,
		"to_storage":      req.ToStorage,
		"files_found":     len(filesToMigrate),
		"files_migrated":  migratedCount,
		"size_migrated":   migratedSize,
		"size_migrated_human": formatBytes(migratedSize),
	}

	if len(errors) > 0 {
		result["errors"] = errors
	}

	if req.DryRun {
		result["message"] = "Dry run completed - no files were actually migrated"
	} else {
		result["message"] = "Migration completed successfully"
	}

	c.JSON(http.StatusOK, result)
}

// Helper function to format bytes
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
