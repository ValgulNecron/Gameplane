package preview

import (
	internalprev "github.com/ValgulNecron/gameplane/gp-module/internal/preview"
)

type PreviewOptions = internalprev.PreviewOptions
type PortPreview = internalprev.PortPreview
type StoragePreview = internalprev.StoragePreview
type ConfigFieldPreview = internalprev.ConfigFieldPreview
type PreviewResult = internalprev.PreviewResult

var (
	GeneratePreview      = internalprev.GeneratePreview
	CalculateAutoMemory = internalprev.CalculateAutoMemory
)
