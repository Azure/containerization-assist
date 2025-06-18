package utils

import (
	"fmt"
	"strings"
)

const progressBarWidth = 30

type ProgressTracker struct {
	spinner []rune
}

func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		spinner: []rune{'|', '/', '-', '\\'},
	}
}

func (pt *ProgressTracker) UpdateProgress(processedFiles, totalFiles int) {
	progress := float64(processedFiles) / float64(totalFiles) * 100
	spinnerChar := pt.spinner[processedFiles%len(pt.spinner)]

	if progress == 100 {
		fmt.Printf("\r        Progress: [%-30s] %c 100.00%%\n", strings.Repeat("=", progressBarWidth), spinnerChar)
	} else {
		fmt.Printf("\r        Progress: [%-30s] %c %.2f%%", strings.Repeat("=", int(progress/float64(100)*float64(progressBarWidth))), spinnerChar, progress)
	}
}
