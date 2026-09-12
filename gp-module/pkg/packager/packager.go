package packager

import (
	internalpack "github.com/ValgulNecron/gameplane/gp-module/internal/packager"
)

type PackageLimits = internalpack.PackageLimits
type PackageWarning = internalpack.PackageWarning
type PackageOptions = internalpack.PackageOptions

var (
	DefaultPackageLimits   = internalpack.DefaultPackageLimits
	CreateArchiveFromFiles = internalpack.CreateArchiveFromFiles
	CreateArchiveFromDir   = internalpack.CreateArchiveFromDir
	ExportArchiveToFile    = internalpack.ExportArchiveToFile
	PushOCI                = internalpack.PushOCI
	ReadModuleFiles        = internalpack.ReadModuleFiles
)
