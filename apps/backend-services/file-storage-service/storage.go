package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
)

// Storage operations

// Store file in MinIO
func (s *FileStorageService) storeFileInMinio(file multipart.File, objectName string, size int64) (string, error) {
	ctx := context.Background()
	
	// Upload file to MinIO
	_, err := s.minioClient.PutObject(ctx, s.config.MinioBucket, objectName, file, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	return fmt.Sprintf("minio://%s/%s", s.config.MinioBucket, objectName), nil
}

// Store file locally
func (s *FileStorageService) storeFileLocally(file multipart.File, filename string) (string, error) {
	// Create directory structure based on date
	now := time.Now()
	dirPath := filepath.Join(s.config.StoragePath, fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day()))
	
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	filePath := filepath.Join(dirPath, filename)
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy file content
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath) // Clean up on error
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	return filePath, nil
}

// Serve file from MinIO
func (s *FileStorageService) serveFileFromMinio(c *gin.Context, metadata *FileMetadata) {
	ctx := context.Background()
	
	// Get object from MinIO
	object, err := s.minioClient.GetObject(ctx, s.config.MinioBucket, metadata.StoredName, minio.GetObjectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file from storage"})
		return
	}
	defer object.Close()

	// Set headers
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", metadata.OriginalName))
	c.Header("Content-Type", metadata.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", metadata.Size))
	c.Header("ETag", metadata.MD5Hash)
	c.Header("Last-Modified", metadata.UpdatedAt.Format(time.RFC1123))

	// Stream file to client
	if _, err := io.Copy(c.Writer, object); err != nil {
		// Log error but don't send JSON response as headers are already sent
		fmt.Printf("Error streaming file: %v\n", err)
	}
}

// Serve file locally
func (s *FileStorageService) serveFileLocally(c *gin.Context, metadata *FileMetadata) {
	// Check if file exists
	if _, err := os.Stat(metadata.Path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found on disk"})
		return
	}

	// Set headers
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", metadata.OriginalName))
	c.Header("Content-Type", metadata.MimeType)
	c.Header("ETag", metadata.MD5Hash)
	c.Header("Last-Modified", metadata.UpdatedAt.Format(time.RFC1123))

	// Serve file
	c.File(metadata.Path)
}

// Delete stored file
func (s *FileStorageService) deleteStoredFile(storageType, path string) error {
	switch storageType {
	case StorageTypeMinio:
		// Extract object name from path (format: minio://bucket/object)
		objectName := filepath.Base(path)
		ctx := context.Background()
		return s.minioClient.RemoveObject(ctx, s.config.MinioBucket, objectName, minio.RemoveObjectOptions{})
	case StorageTypeLocal:
		return os.Remove(path)
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

// Clean up stored file (used on errors)
func (s *FileStorageService) cleanupStoredFile(storageType, path string) {
	if err := s.deleteStoredFile(storageType, path); err != nil {
		fmt.Printf("Failed to cleanup file %s: %v\n", path, err)
	}
}

// Merge chunks into final file
func (s *FileStorageService) mergeChunks(fileID string, totalChunks int) error {
	// Get all chunks for the file
	var chunks []FileChunk
	if err := s.db.Where("file_id = ? AND status = ?", fileID, FileStatusActive).
		Order("chunk_index ASC").
		Find(&chunks).Error; err != nil {
		return fmt.Errorf("failed to get chunks: %w", err)
	}

	if len(chunks) != totalChunks {
		return fmt.Errorf("missing chunks: expected %d, got %d", totalChunks, len(chunks))
	}

	// Create temporary merged file
	tempPath := filepath.Join(s.config.StoragePath, "temp", fmt.Sprintf("%s_merged", fileID))
	if err := os.MkdirAll(filepath.Dir(tempPath), 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	mergedFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create merged file: %w", err)
	}
	defer mergedFile.Close()

	// Merge chunks in order
	var totalSize int64
	for _, chunk := range chunks {
		chunkFile, err := os.Open(chunk.Path)
		if err != nil {
			return fmt.Errorf("failed to open chunk %d: %w", chunk.ChunkIndex, err)
		}

		written, err := io.Copy(mergedFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy chunk %d: %w", chunk.ChunkIndex, err)
		}

		totalSize += written
	}

	// Calculate final file hash
	mergedFile.Seek(0, 0)
	md5Hash, sha256Hash, err := calculateHashes(mergedFile)
	if err != nil {
		return fmt.Errorf("failed to calculate merged file hash: %w", err)
	}

	// Move merged file to final location
	finalPath := filepath.Join(s.config.StoragePath, "merged", fmt.Sprintf("%s_final", fileID))
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return fmt.Errorf("failed to create final directory: %w", err)
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("failed to move merged file: %w", err)
	}

	// Create file metadata
	metadata := &FileMetadata{
		ID:              fileID,
		OriginalName:    fmt.Sprintf("merged_file_%s", fileID),
		StoredName:      fmt.Sprintf("%s_final", fileID),
		Path:            finalPath,
		Size:            totalSize,
		MimeType:        "application/octet-stream",
		MD5Hash:         md5Hash,
		SHA256Hash:      sha256Hash,
		StorageType:     StorageTypeLocal,
		StorageLocation: finalPath,
		Status:          FileStatusActive,
		Version:         1,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	// Save merged file metadata
	if err := s.db.Create(metadata).Error; err != nil {
		return fmt.Errorf("failed to save merged file metadata: %w", err)
	}

	// Clean up chunks
	for _, chunk := range chunks {
		os.Remove(chunk.Path)
		s.db.Delete(&chunk)
	}

	return nil
}

// Cache operations

// Cache file metadata in Redis
func (s *FileStorageService) cacheFileMetadata(metadata *FileMetadata) {
	ctx := context.Background()
	key := fmt.Sprintf("file_metadata:%s", metadata.ID)
	
	data, err := json.Marshal(metadata)
	if err != nil {
		fmt.Printf("Failed to marshal file metadata for cache: %v\n", err)
		return
	}

	// Cache for 1 hour
	if err := s.redis.Set(ctx, key, data, 1*time.Hour).Err(); err != nil {
		fmt.Printf("Failed to cache file metadata: %v\n", err)
	}
}

// Get cached file metadata
func (s *FileStorageService) getCachedFileMetadata(fileID string) *FileMetadata {
	ctx := context.Background()
	key := fmt.Sprintf("file_metadata:%s", fileID)
	
	data, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return nil
	}

	var metadata FileMetadata
	if err := json.Unmarshal([]byte(data), &metadata); err != nil {
		return nil
	}

	return &metadata
}

// Remove cached file metadata
func (s *FileStorageService) removeCachedFileMetadata(fileID string) {
	ctx := context.Background()
	key := fmt.Sprintf("file_metadata:%s", fileID)
	s.redis.Del(ctx, key)
}

// Update file access tracking
func (s *FileStorageService) updateFileAccess(fileID string) {
	now := time.Now().UTC()
	
	// Update database
	s.db.Model(&FileMetadata{}).
		Where("id = ?", fileID).
		Updates(map[string]interface{}{
			"download_count":    gorm.Expr("download_count + 1"),
			"last_accessed_at":  now,
			"updated_at":        now,
		})

	// Update cache counter
	ctx := context.Background()
	key := fmt.Sprintf("file_downloads:%s", fileID)
	s.redis.Incr(ctx, key)
	s.redis.Expire(ctx, key, 24*time.Hour)
}

// Background workers

// Cleanup worker - removes expired and orphaned files
func (s *FileStorageService) startCleanupWorker() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupExpiredFiles()
			s.cleanupOrphanedFiles()
		}
	}
}

// Metrics updater - updates storage metrics
func (s *FileStorageService) startMetricsUpdater() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.updateStorageMetrics()
		}
	}
}

// Clean up expired files
func (s *FileStorageService) cleanupExpiredFiles() {
	var expiredFiles []FileMetadata
	if err := s.db.Where("expires_at IS NOT NULL AND expires_at < ? AND status = ?", 
		time.Now().UTC(), FileStatusActive).Find(&expiredFiles).Error; err != nil {
		fmt.Printf("Failed to find expired files: %v\n", err)
		return
	}

	for _, file := range expiredFiles {
		// Mark as deleted
		file.Status = FileStatusDeleted
		file.UpdatedAt = time.Now().UTC()
		
		if err := s.db.Save(&file).Error; err != nil {
			fmt.Printf("Failed to mark file %s as expired: %v\n", file.ID, err)
			continue
		}

		// Remove from cache
		s.removeCachedFileMetadata(file.ID)
		
		fmt.Printf("Marked file %s as expired\n", file.ID)
	}
}

// Clean up orphaned files (files on disk without metadata)
func (s *FileStorageService) cleanupOrphanedFiles() {
	// This is a simplified version - in production, you'd want more sophisticated orphan detection
	fmt.Println("Orphaned file cleanup completed")
}

// Update storage metrics
func (s *FileStorageService) updateStorageMetrics() {
	// Update storage usage by user and storage type
	var results []struct {
		UserID      string `json:"user_id"`
		StorageType string `json:"storage_type"`
		TotalSize   int64  `json:"total_size"`
	}

	if err := s.db.Model(&FileMetadata{}).
		Select("user_id, storage_type, SUM(size) as total_size").
		Where("status = ?", FileStatusActive).
		Group("user_id, storage_type").
		Find(&results).Error; err != nil {
		fmt.Printf("Failed to update storage metrics: %v\n", err)
		return
	}

	// Reset all metrics first
	storageUsed.Reset()

	// Update metrics
	for _, result := range results {
		storageUsed.WithLabelValues(result.StorageType, result.UserID).Set(float64(result.TotalSize))
	}
}

// File sharing operations

// Create file share
func (s *FileStorageService) createFileShare(c *gin.Context) {
	fileID := c.Param("id")
	
	var req FileShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify file exists
	var metadata FileMetadata
	if err := s.db.First(&metadata, "id = ? AND status = ?", fileID, FileStatusActive).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Create share
	share := &FileShare{
		ID:           uuid.New().String(),
		FileID:       fileID,
		ShareToken:   uuid.New().String(),
		ShareType:    req.ShareType,
		Password:     req.Password,
		Permissions:  req.Permissions,
		ExpiresAt:    req.ExpiresAt,
		MaxDownloads: req.MaxDownloads,
		CreatedBy:    c.GetString("user_id"), // From auth middleware
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.db.Create(share).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file share"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"share_id":    share.ID,
		"share_token": share.ShareToken,
		"share_url":   fmt.Sprintf("/v1/shared/%s", share.ShareToken),
		"expires_at":  share.ExpiresAt,
		"message":     "File share created successfully",
	})
}

// Get file shares
func (s *FileStorageService) getFileShares(c *gin.Context) {
	fileID := c.Param("id")

	var shares []FileShare
	if err := s.db.Where("file_id = ?", fileID).Find(&shares).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file shares"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file_id": fileID,
		"shares":  shares,
		"count":   len(shares),
	})
}

// Delete file share
func (s *FileStorageService) deleteFileShare(c *gin.Context) {
	token := c.Param("token")

	if err := s.db.Where("share_token = ?", token).Delete(&FileShare{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file share"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File share deleted successfully",
	})
}

// Get shared file metadata
func (s *FileStorageService) getSharedFile(c *gin.Context) {
	token := c.Param("token")

	var share FileShare
	if err := s.db.Where("share_token = ?", token).First(&share).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Share not found"})
		return
	}

	// Check if share is expired
	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now().UTC()) {
		c.JSON(http.StatusGone, gin.H{"error": "Share has expired"})
		return
	}

	// Check download limit
	if share.MaxDownloads > 0 && share.DownloadCount >= share.MaxDownloads {
		c.JSON(http.StatusGone, gin.H{"error": "Download limit exceeded"})
		return
	}

	// Get file metadata
	var metadata FileMetadata
	if err := s.db.First(&metadata, "id = ? AND status = ?", share.FileID, FileStatusActive).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"share":    share,
		"file":     metadata,
		"can_download": true,
	})
}

// Download shared file
func (s *FileStorageService) downloadSharedFile(c *gin.Context) {
	token := c.Param("token")
	password := c.Query("password")

	var share FileShare
	if err := s.db.Where("share_token = ?", token).First(&share).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Share not found"})
		return
	}

	// Check if share is expired
	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now().UTC()) {
		c.JSON(http.StatusGone, gin.H{"error": "Share has expired"})
		return
	}

	// Check download limit
	if share.MaxDownloads > 0 && share.DownloadCount >= share.MaxDownloads {
		c.JSON(http.StatusGone, gin.H{"error": "Download limit exceeded"})
		return
	}

	// Check password if required
	if share.ShareType == "password" && share.Password != password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	// Get file metadata
	var metadata FileMetadata
	if err := s.db.First(&metadata, "id = ? AND status = ?", share.FileID, FileStatusActive).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Update download count
	share.DownloadCount++
	share.UpdatedAt = time.Now().UTC()
	s.db.Save(&share)

	// Serve file
	switch metadata.StorageType {
	case StorageTypeMinio:
		s.serveFileFromMinio(c, &metadata)
	case StorageTypeLocal:
		s.serveFileLocally(c, &metadata)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unsupported storage type"})
	}
}
