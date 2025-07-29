package scanner

import (
	"fmt"
	"strings"

	"github.com/kevinfinalboss/privateer/pkg/types"
)

func (fs *FileScanner) scanKustomization(content, filePath string, publicImageMap map[string]*types.ImageInfo) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult
	lines := strings.Split(content, "\n")

	var currentNewName string
	var currentNewTag string

	for lineNum, line := range lines {
		if matches := imagePatterns["kustomize_newName"].FindStringSubmatch(line); len(matches) > 1 {
			currentNewName = matches[1]
		}

		if matches := imagePatterns["kustomize_newTag"].FindStringSubmatch(line); len(matches) > 1 {
			currentNewTag = matches[1]
		}

		if currentNewName != "" && currentNewTag != "" {
			fullImage := fmt.Sprintf("%s:%s", currentNewName, currentNewTag)

			if _, isPublic := publicImageMap[fullImage]; isPublic {
				detections = append(detections, types.ImageDetectionResult{
					Image:      fullImage,
					Repository: currentNewName,
					Tag:        currentNewTag,
					Registry:   fs.extractRegistry(currentNewName),
					FullImage:  fullImage,
					IsPublic:   true,
					LineNumber: lineNum + 1,
					Context:    fmt.Sprintf("newName: %s, newTag: %s", currentNewName, currentNewTag),
					Confidence: 0.9,
				})

				fs.logger.Debug("kustomize_image_detected").
					Str("file", filePath).
					Str("newName", currentNewName).
					Str("newTag", currentNewTag).
					Send()
			}

			currentNewName = ""
			currentNewTag = ""
		}
	}

	return detections
}
