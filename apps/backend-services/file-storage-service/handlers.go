package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

// Request/Response types
type UploadResponse struct {
	FileID       string            `json:"file_id"`
	OriginalName string            `json:"original_name"`
	Size         int64             `json:"size"`
	MimeType     string            `json:"mime_type"`
	MD5Hash      string            `json:"md5_hash"`
	SHA256Hash   string            `json:"sha256_hash"`
	StorageType  string            `json:"storage_type"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	UploadTime   time.Duration     `json:"upload_time_ms"`
}

type FileShareRequest struct {
	ShareType    string     `json:"share_type" binding:"required"` // public, private, password
	Password     string     `json:"password,omitempty"`
	Permissions  []string   `json:"permissions"`
	ExpiresAt    *time.Time `json:"expires_at"`
	MaxDownloads int        `json:"max_downloads"`
}

type BatchOperationRequest struct {
	FileIDs []string `json:"file_ids" binding:"required"`
}

type BatchMoveRequest struct {
	FileIDs     []string          `json:"file_ids" binding:"required"`
	Destination string            `json:"destination" binding:"required"`
	Metadata    map[string]string `json:"metadata"`
}

// File upload handler
func (s *FileStorageService) uploadFile(c *gin.Context) {
	start := time.Now()

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	// Check file size
	if header.Size > s.config.MaxFileSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error":        "File too large",
			"max_size":     s.config.MaxFileSize,
			"actual_size":  header.Size,
		})
		return
	}

	// Get additional metadata from form
	userID := c.PostForm("user_id")
	projectID := c.PostForm("project_id")
	tags := strings.Split(c.PostForm("tags"), ",")
	storageType := c.DefaultPostForm("storage_type", StorageTypeMinio)

	// Calculate file hashes
	md5Hash, sha256Hash, err := calculateHashes(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate file hashes"})
		return
	}

	// Check for duplicates
	var existingFile FileMetadata
	if err := s.db.Where("md5_hash = ? AND status = ?", md5Hash, FileStatusActive).First(&existingFile).Error; err == nil {
		// File already exists, return existing metadata
		c.JSON(http.StatusOK, gin.H{
			"file_id":      existingFile.ID,
			"message":      "File already exists",
			"existing":     true,
			"original_id":  existingFile.ID,
		})
		return
	}

	// Create file metadata
	fileID := uuid.New().String()
	extension := filepath.Ext(header.Filename)
	storedName := fmt.Sprintf("%s%s", fileID, extension)
	
	metadata := &FileMetadata{
		ID:           fileID,
		OriginalName: header.Filename,
		StoredName:   storedName,
		Size:         header.Size,
		MimeType:     header.Header.Get("Content-Type"),
		Extension:    extension,
		MD5Hash:      md5Hash,
		SHA256Hash:   sha256Hash,
		StorageType:  storageType,
		Status:       FileStatusUploading,
		Version:      1,
		UserID:       userID,
		ProjectID:    projectID,
		Tags:         tags,
		Metadata:     make(map[string]string),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Add custom metadata from form
	for key, values := range c.Request.PostForm {
		if strings.HasPrefix(key, "meta_") {
			metaKey := strings.TrimPrefix(key, "meta_")
			if len(values) > 0 {
				metadata.Metadata[metaKey] = values[0]
			}
		}
	}

	// Store file based on storage type
	var storagePath string
	switch storageType {
	case StorageTypeMinio:
		storagePath, err = s.storeFileInMinio(file, storedName, header.Size)
	case StorageTypeLocal:
		storagePath, err = s.storeFileLocally(file, storedName)
	default:
		err = fmt.Errorf("unsupported storage type: %s", storageType)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store file"})
		return
	}

	metadata.Path = storagePath
	metadata.StorageLocation = storagePath
	metadata.Status = FileStatusActive

	// Save metadata to database
	if err := s.db.Create(metadata).Error; err != nil {
		// Clean up stored file on database error
		s.cleanupStoredFile(storageType, storagePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
		return
	}

	// Update metrics
	sizeCategory := getSizeCategory(header.Size)
	filesUploaded.WithLabelValues(storageType, metadata.MimeType).Inc()
	uploadDuration.WithLabelValues(storageType, sizeCategory).Observe(time.Since(start).Seconds())
	storageUsed.WithLabelValues(storageType, userID).Add(float64(header.Size))

	// Cache file metadata
	go s.cacheFileMetadata(metadata)

	uploadTime := time.Since(start)
	response := UploadResponse{
		FileID:       fileID,
		OriginalName: header.Filename,
		Size:         header.Size,
		MimeType:     metadata.MimeType,
		MD5Hash:      md5Hash,
		SHA256Hash:   sha256Hash,
		StorageType:  storageType,
		Metadata:     metadata.Metadata,
		UploadTime:   uploadTime,
	}

	c.JSON(http.StatusCreated, response)
}

// Chunked file upload handler
func (s *FileStorageService) uploadChunkedFile(c *gin.Context) {
	fileID := c.PostForm("file_id")
	chunkIndex, _ := strconv.Atoi(c.PostForm("chunk_index"))
	totalChunks, _ := strconv.Atoi(c.PostForm("total_chunks"))
	
	if fileID == "" {
		fileID = uuid.New().String()
	}

	// Parse chunk file
	file, header, err := c.Request.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No chunk provided"})
		return
	}
	defer file.Close()

	// Calculate chunk hash
	md5Hash, _, err := calculateHashes(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate chunk hash"})
		return
	}

	// Store chunk
	chunkID := uuid.New().String()
	chunkName := fmt.Sprintf("%s_chunk_%d", fileID, chunkIndex)
	chunkPath, err := s.storeFileLocally(file, chunkName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store chunk"})
		return
	}

	// Save chunk metadata
	chunk := &FileChunk{
		ID:         chunkID,
		FileID:     fileID,
		ChunkIndex: chunkIndex,
		Size:       header.Size,
		MD5Hash:    md5Hash,
		Path:       chunkPath,
		Status:     FileStatusActive,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	if err := s.db.Create(chunk).Error; err != nil {
		os.Remove(chunkPath) // Clean up chunk file
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save chunk metadata"})
		return
	}

	// Check if all chunks are uploaded
	var uploadedChunks int64
	s.db.Model(&FileChunk{}).Where("file_id = ? AND status = ?", fileID, FileStatusActive).Count(&uploadedChunks)

	response := gin.H{
		"file_id":         fileID,
		"chunk_id":        chunkID,
		"chunk_index":     chunkIndex,
		"uploaded_chunks": uploadedChunks,
		"total_chunks":    totalChunks,
	}

	if int(uploadedChunks) == totalChunks {
		// All chunks uploaded, merge them
		go s.mergeChunks(fileID, totalChunks)
		response["status"] = "complete"
		response["message"] = "All chunks uploaded, merging in progress"
	} else {
		response["status"] = "partial"
		response["message"] = "Chunk uploaded successfully"
	}

	c.JSON(http.StatusOK, response)
}

// Get file metadata
func (s *FileStorageService) getFileMetadata(c *gin.Context) {
	fileID := c.Param("id")

	// Try cache first
	if metadata := s.getCachedFileMetadata(fileID); metadata != nil {
		c.JSON(http.StatusOK, metadata)
		return
	}

	// Get from database
	var metadata FileMetadata
	if err := s.db.First(&metadata, "id = ? AND status != ?", fileID, FileStatusDeleted).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Cache for future requests
	go s.cacheFileMetadata(&metadata)

	c.JSON(http.StatusOK, metadata)
}

// Download file
func (s *FileStorageService) downloadFile(c *gin.Context) {
	start := time.Now()
	fileID := c.Param("id")

	// Get file metadata
	var metadata FileMetadata
	if err := s.db.First(&metadata, "id = ? AND status = ?", fileID, FileStatusActive).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Update access tracking
	go s.updateFileAccess(fileID)

	// Serve file based on storage type
	switch metadata.StorageType {
	case StorageTypeMinio:
		s.serveFileFromMinio(c, &metadata)
	case StorageTypeLocal:
		s.serveFileLocally(c, &metadata)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unsupported storage type"})
		return
	}

	// Update metrics
	sizeCategory := getSizeCategory(metadata.Size)
	filesDownloaded.WithLabelValues(metadata.StorageType).Inc()
	downloadDuration.WithLabelValues(metadata.StorageType, sizeCategory).Observe(time.Since(start).Seconds())
}

// Update file metadata
func (s *FileStorageService) updateFileMetadata(c *gin.Context) {
	fileID := c.Param("id")

	var req struct {
		OriginalName string            `json:"original_name"`
		Tags         []string          `json:"tags"`
		Metadata     map[string]string `json:"metadata"`
		ExpiresAt    *time.Time        `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing file
	var metadata FileMetadata
	if err := s.db.First(&metadata, "id = ? AND status != ?", fileID, FileStatusDeleted).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Update fields
	if req.OriginalName != "" {
		metadata.OriginalName = req.OriginalName
	}
	if req.Tags != nil {
		metadata.Tags = req.Tags
	}
	if req.Metadata != nil {
		metadata.Metadata = req.Metadata
	}
	if req.ExpiresAt != nil {
		metadata.ExpiresAt = req.ExpiresAt
	}
	metadata.UpdatedAt = time.Now().UTC()

	if err := s.db.Save(&metadata).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update file metadata"})
		return
	}

	// Update cache
	go s.cacheFileMetadata(&metadata)

	c.JSON(http.StatusOK, gin.H{
		"message": "File metadata updated successfully",
		"file":    metadata,
	})
}

// Delete file
func (s *FileStorageService) deleteFile(c *gin.Context) {
	fileID := c.Param("id")
	permanent := c.Query("permanent") == "true"

	var metadata FileMetadata
	if err := s.db.First(&metadata, "id = ? AND status != ?", fileID, FileStatusDeleted).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	if permanent {
		// Permanently delete file
		if err := s.deleteStoredFile(metadata.StorageType, metadata.Path); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete stored file"})
			return
		}

		// Delete from database
		if err := s.db.Delete(&metadata).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file metadata"})
			return
		}

		// Update storage metrics
		storageUsed.WithLabelValues(metadata.StorageType, metadata.UserID).Sub(float64(metadata.Size))
	} else {
		// Soft delete
		metadata.Status = FileStatusDeleted
		metadata.UpdatedAt = time.Now().UTC()

		if err := s.db.Save(&metadata).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark file as deleted"})
			return
		}
	}

	// Remove from cache
	go s.removeCachedFileMetadata(fileID)

	c.JSON(http.StatusOK, gin.H{
		"message":   "File deleted successfully",
		"permanent": permanent,
	})
}

// Create file version
func (s *FileStorageService) createFileVersion(c *gin.Context) {
	parentID := c.Param("id")

	// Get parent file
	var parentFile FileMetadata
	if err := s.db.First(&parentFile, "id = ? AND status = ?", parentID, FileStatusActive).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent file not found"})
		return
	}

	// Parse new file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	// Calculate hashes
	md5Hash, sha256Hash, err := calculateHashes(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate file hashes"})
		return
	}

	// Get next version number
	var maxVersion int
	s.db.Model(&FileMetadata{}).
		Where("parent_id = ? OR id = ?", parentID, parentID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion)

	// Create new version
	versionID := uuid.New().String()
	extension := filepath.Ext(header.Filename)
	storedName := fmt.Sprintf("%s_v%d%s", versionID, maxVersion+1, extension)

	// Store file
	storagePath, err := s.storeFileInMinio(file, storedName, header.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store file version"})
		return
	}

	// Create version metadata
	versionMetadata := &FileMetadata{
		ID:              versionID,
		OriginalName:    header.Filename,
		StoredName:      storedName,
		Path:            storagePath,
		Size:            header.Size,
		MimeType:        header.Header.Get("Content-Type"),
		Extension:       extension,
		MD5Hash:         md5Hash,
		SHA256Hash:      sha256Hash,
		StorageType:     parentFile.StorageType,
		StorageLocation: storagePath,
		Status:          FileStatusActive,
		Version:         maxVersion + 1,
		ParentID:        parentID,
		UserID:          parentFile.UserID,
		ProjectID:       parentFile.ProjectID,
		Tags:            parentFile.Tags,
		Metadata:        parentFile.Metadata,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.db.Create(versionMetadata).Error; err != nil {
		s.cleanupStoredFile(parentFile.StorageType, storagePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save version metadata"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"version_id": versionID,
		"version":    maxVersion + 1,
		"parent_id":  parentID,
		"message":    "File version created successfully",
	})
}

// Get file versions
func (s *FileStorageService) getFileVersions(c *gin.Context) {
	fileID := c.Param("id")

	var versions []FileMetadata
	if err := s.db.Where("parent_id = ? OR id = ?", fileID, fileID).
		Order("version ASC").
		Find(&versions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file versions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file_id":  fileID,
		"versions": versions,
		"count":    len(versions),
	})
}

// List files
func (s *FileStorageService) listFiles(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	userID := c.Query("user_id")
	projectID := c.Query("project_id")
	mimeType := c.Query("mime_type")
	status := c.DefaultQuery("status", FileStatusActive)

	query := s.db.Model(&FileMetadata{}).Where("status = ?", status)

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if mimeType != "" {
		query = query.Where("mime_type LIKE ?", mimeType+"%")
	}

	var total int64
	query.Count(&total)

	var files []FileMetadata
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files":  files,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Search files
func (s *FileStorageService) searchFiles(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var files []FileMetadata
	if err := s.db.Where("status = ? AND (original_name ILIKE ? OR tags::text ILIKE ?)", 
		FileStatusActive, "%"+query+"%", "%"+query+"%").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"files":   files,
		"count":   len(files),
		"limit":   limit,
		"offset":  offset,
	})
}

// Find duplicate files
func (s *FileStorageService) findDuplicates(c *gin.Context) {
	var duplicates []struct {
		MD5Hash string `json:"md5_hash"`
		Count   int64  `json:"count"`
		Size    int64  `json:"size"`
	}

	if err := s.db.Model(&FileMetadata{}).
		Select("md5_hash, COUNT(*) as count, size").
		Where("status = ?", FileStatusActive).
		Group("md5_hash, size").
		Having("COUNT(*) > 1").
		Find(&duplicates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find duplicates"})
		return
	}

	// Get detailed information for each duplicate group
	var result []gin.H
	for _, dup := range duplicates {
		var files []FileMetadata
		s.db.Where("md5_hash = ? AND status = ?", dup.MD5Hash, FileStatusActive).Find(&files)
		
		result = append(result, gin.H{
			"md5_hash":    dup.MD5Hash,
			"count":       dup.Count,
			"size":        dup.Size,
			"total_waste": (dup.Count - 1) * dup.Size,
			"files":       files,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"duplicates": result,
		"groups":     len(result),
	})
}
