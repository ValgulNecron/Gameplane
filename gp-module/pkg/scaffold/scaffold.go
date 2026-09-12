// Package scaffold provides public scaffold exports.
package scaffold

import (
	internalscaffold "github.com/ValgulNecron/gameplane/gp-module/internal/scaffold"
)

type ScaffoldOptions = internalscaffold.ScaffoldOptions
type GeneratedFiles = internalscaffold.GeneratedFiles
type ScaffoldResult = internalscaffold.ScaffoldResult

var (
	GenerateFiles = internalscaffold.GenerateFiles
	Scaffold      = internalscaffold.Scaffold
)
