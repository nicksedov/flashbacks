package helpers

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/domain"

	"gorm.io/gorm"
)

// FileDeletionResult holds the result of a batch file deletion operation.
type FileDeletionResult struct {
	Success     int
	Failed      int
	FailedFiles []string
}

// FileMover handles file deletion and trash operations.
type FileMover struct {
	db *gorm.DB
}

// NewFileMover creates a new FileMover.
func NewFileMover(db *gorm.DB) *FileMover {
	return &FileMover{db: db}
}

// MoveToTrashOrDelete moves a file to trash if trashDir is set, otherwise permanently deletes it.
func (fm *FileMover) MoveToTrashOrDelete(filePath, trashDir string) error {
	if trashDir != "" {
		if err := os.MkdirAll(trashDir, 0755); err != nil {
			return err
		}
		return fm.moveToTrash(filePath, trashDir)
	}
	return os.Remove(filePath)
}

// DeleteFromDB removes the image file record from the database.
func (fm *FileMover) DeleteFromDB(filePath string) {
	fm.db.Where("path = ?", filepath.ToSlash(filePath)).Delete(&domain.ImageFile{})
}

// BatchProcess processes multiple files: moves to trash or deletes, and removes from DB.
func (fm *FileMover) BatchProcess(filePaths []string, trashDir string) FileDeletionResult {
	var result FileDeletionResult

	if trashDir != "" {
		if err := os.MkdirAll(trashDir, 0755); err != nil {
			result.Failed = len(filePaths)
			for _, fp := range filePaths {
				result.FailedFiles = append(result.FailedFiles, filepath.Base(fp)+": "+err.Error())
			}
			return result
		}
	}

	for _, filePath := range filePaths {
		baseName := filepath.Base(filePath)

		if err := fm.MoveToTrashOrDelete(filePath, trashDir); err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, baseName+": "+err.Error())
			continue
		}

		fm.DeleteFromDB(filePath)
		result.Success++
	}

	return result
}

// BatchProcessWithRules processes files based on keep-folder rules (for batch dedup deletion).
// Only deletes files NOT in the keepFolder for each file.
func (fm *FileMover) BatchProcessWithRules(files []struct{ Path string }, keepFolder, trashDir string) FileDeletionResult {
	var result FileDeletionResult

	if trashDir != "" {
		if err := os.MkdirAll(trashDir, 0755); err != nil {
			result.Failed = len(files)
			return result
		}
	}

	for _, file := range files {
		fileDir := filepath.Dir(file.Path)
		if fileDir == keepFolder {
			continue
		}

		baseName := filepath.Base(file.Path)
		if err := fm.MoveToTrashOrDelete(file.Path, trashDir); err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, baseName+": "+err.Error())
			continue
		}

		fm.DeleteFromDB(file.Path)
		result.Success++
	}

	return result
}

// ValidateDirectory validates that a path exists, is a directory, and returns its normalized form.
func ValidateDirectory(path string) (normalized string, err error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", os.ErrInvalid
	}

	return filepath.ToSlash(absPath), nil
}

// CheckPathsConflict checks if two normalized paths conflict (same, parent/child).
func CheckPathsConflict(a, b string) bool {
	return pathsConflict(a, b) != ""
}

func (fm *FileMover) moveToTrash(filePath, trashDir string) error {
	baseName := filepath.Base(filePath)
	destPath := filepath.Join(trashDir, baseName)

	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(baseName)
		nameWithoutExt := strings.TrimSuffix(baseName, ext)
		destPath = filepath.Join(trashDir, nameWithoutExt+"_"+time.Now().Format(TrashTimestampFormat)+ext)
	}

	return os.Rename(filePath, destPath)
}

func pathsConflict(a, b string) string {
	na := strings.TrimRight(strings.ToLower(a), "/")
	nb := strings.TrimRight(strings.ToLower(b), "/")

	if na == nb {
		return "same"
	}
	if strings.HasPrefix(na, nb+"/") {
		return "child"
	}
	if strings.HasPrefix(nb, na+"/") {
		return "parent"
	}
	return ""
}

// RestoreFile restores a file from trash to a target path, handling duplicates.
func RestoreFile(trashDir, fileName, targetPath string) (restoredPath string, err error) {
	trashPath := filepath.Join(trashDir, fileName)
	if _, err := os.Stat(trashPath); err != nil {
		return "", err
	}

	if targetPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		targetPath = filepath.Join(cwd, fileName)
	}

	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	if _, err := os.Stat(targetPath); err == nil {
		ext := filepath.Ext(fileName)
		nameWithoutExt := strings.TrimSuffix(fileName, ext)
		targetPath = nameWithoutExt + "_restored_" + time.Now().Format(TrashTimestampFormat) + ext
	}

	err = os.Rename(trashPath, targetPath)
	if err != nil {
		srcFile, openErr := os.Open(trashPath)
		if openErr != nil {
			return "", openErr
		}
		defer srcFile.Close()

		dstFile, createErr := os.Create(targetPath)
		if createErr != nil {
			return "", createErr
		}
		defer dstFile.Close()

		if _, copyErr := io.Copy(dstFile, srcFile); copyErr != nil {
			dstFile.Close()
			os.Remove(targetPath)
			return "", copyErr
		}

		os.Remove(trashPath)
	}

	return targetPath, nil
}
