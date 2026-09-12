package archetypes

import (
	internalarch "github.com/ValgulNecron/gameplane/gp-module/internal/archetypes"
)

type PortDef = internalarch.PortDef
type StorageDef = internalarch.StorageDef
type ArchetypeDefinition = internalarch.ArchetypeDefinition

var (
	GetArchetype         = internalarch.GetArchetype
	AllArchetypes        = internalarch.AllArchetypes
	PlaceholderIconBytes = internalarch.PlaceholderIconBytes
)
