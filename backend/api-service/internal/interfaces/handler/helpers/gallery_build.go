package helpers

import (
	"path/filepath"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
)

// BuildGalleryImageDTO converts a domain.ImageFile to a GalleryImageDTO with
// computed fields (FileName, DirPath, SizeHuman, ModTime).
func BuildGalleryImageDTO(f *domain.ImageFile) dto.GalleryImageDTO {
	return dto.GalleryImageDTO{
		ID:        f.ID,
		Path:      f.Path,
		FileName:  filepath.Base(f.Path),
		DirPath:   filepath.Dir(f.Path),
		Size:      f.Size,
		SizeHuman: FormatSize(f.Size),
		ModTime:   f.ModTime.Format(DateTimeFormat),
	}
}

// BuildFileDTO converts a domain.ImageFile to a dto.FileDTO.
func BuildFileDTO(f *domain.ImageFile) dto.FileDTO {
	return dto.FileDTO{
		ID:       f.ID,
		Path:     f.Path,
		FileName: filepath.Base(f.Path),
		DirPath:  filepath.Dir(f.Path),
		ModTime:  f.ModTime.Format(DateTimeFormat),
	}
}

// BuildGalleryImageDTOs converts a slice of domain.ImageFile to GalleryImageDTOs.
func BuildGalleryImageDTOs(files []domain.ImageFile) []dto.GalleryImageDTO {
	dtos := make([]dto.GalleryImageDTO, len(files))
	for i, f := range files {
		dtos[i] = BuildGalleryImageDTO(&f)
	}
	return dtos
}