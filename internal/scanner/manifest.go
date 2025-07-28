package scanner

import (
	"strings"

	"github.com/kevinfinalboss/privateer/pkg/types"
)

func (fs *FileScanner) scanKubernetesManifest(content, filePath string, publicImageMap map[string]*types.ImageInfo) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		if matches := imagePatterns["yaml_image"].FindStringSubmatch(line); len(matches) > 1 {
			imageName := matches[1]
			if _, isPublic := publicImageMap[imageName]; isPublic {
				detections = append(detections, types.ImageDetectionResult{
					Image:      imageName,
					Repository: fs.extractRepository(imageName),
					Tag:        fs.extractTag(imageName),
					Registry:   fs.extractRegistry(imageName),
					FullImage:  imageName,
					IsPublic:   true,
					LineNumber: lineNum + 1,
					Context:    strings.TrimSpace(line),
					Confidence: 1.0,
					FilePath:   filePath,
				})

				fs.logger.Debug("kubernetes_image_detected").
					Str("file", filePath).
					Str("image", imageName).
					Int("line", lineNum+1).
					Send()
			}
		}
	}

	return detections
}
