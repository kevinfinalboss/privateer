package logger

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

func NewTest() *Logger {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	testLogger := zerolog.New(io.Discard).With().Timestamp().Logger()

	l := &Logger{
		logger:   testLogger,
		language: "en-US",
		messages: make(map[string]string),
	}

	l.messages = map[string]string{
		"scanning_namespace":                      "Scanning namespace",
		"namespace_scan_summary_before_filtering": "Namespace scan summary before filtering",
		"image_found_before_filtering":            "Image found before filtering",
		"images_found":                            "Images found",
		"resource_scanned":                        "Resource scanned",
		"starting_image_filtering":                "Starting image filtering",
		"analyzing_image_publicity":               "Analyzing image publicity",
		"image_publicity_result":                  "Image publicity result",
		"image_added_to_public_list":              "Image added to public list",
		"image_excluded_from_public_list":         "Image excluded from public list",
		"image_filtering_completed":               "Image filtering completed",
		"starting_image_classification":           "Starting image classification",
		"image_classification_result":             "Image classification result",
		"ignore_registry_check":                   "Ignore registry check",
		"custom_private_registry_check":           "Custom private registry check",
		"custom_public_registry_check":            "Custom public registry check",
		"private_registry_detection_start":        "Private registry detection start",
		"private_registry_detection":              "Private registry detection",
	}

	return l
}

func NewTestWithOutput() *Logger {
	testLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	l := &Logger{
		logger:   testLogger,
		language: "en-US",
		messages: make(map[string]string),
	}

	l.messages = map[string]string{
		"scanning_namespace":                      "Scanning namespace",
		"namespace_scan_summary_before_filtering": "Namespace scan summary before filtering",
		"image_found_before_filtering":            "Image found before filtering",
		"images_found":                            "Images found",
		"resource_scanned":                        "Resource scanned",
		"starting_image_filtering":                "Starting image filtering",
		"analyzing_image_publicity":               "Analyzing image publicity",
		"image_publicity_result":                  "Image publicity result",
		"image_added_to_public_list":              "Image added to public list",
		"image_excluded_from_public_list":         "Image excluded from public list",
		"image_filtering_completed":               "Image filtering completed",
		"starting_image_classification":           "Starting image classification",
		"image_classification_result":             "Image classification result",
		"ignore_registry_check":                   "Ignore registry check",
		"custom_private_registry_check":           "Custom private registry check",
		"custom_public_registry_check":            "Custom public registry check",
		"private_registry_detection_start":        "Private registry detection start",
		"private_registry_detection":              "Private registry detection",
	}

	return l
}
