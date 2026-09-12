// Package validator provides public validator exports.
package validator

import (
	internalval "github.com/ValgulNecron/gameplane/gp-module/internal/validator"
)

type Severity = internalval.Severity

const (
	SeverityError = internalval.SeverityError
	SeverityWarn  = internalval.SeverityWarn
)

type ValidateOptions = internalval.ValidateOptions
type ModuleReport = internalval.ModuleReport
type Finding = internalval.Finding
type ValidationReport = internalval.ValidationReport

var (
	ValidateDirectory = internalval.ValidateDirectory
	ValidateFiles     = internalval.ValidateFiles
)
