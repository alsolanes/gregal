// Package config reexporta la configuració de Gregal per a productes externs
// (vegeu gregal/host).
package config

import iconfig "gregal/internal/config"

type (
	AgentCfg       = iconfig.AgentCfg
	Config         = iconfig.Config
	ImportedModel  = iconfig.ImportedModel
	PermissionsCfg = iconfig.PermissionsCfg
	Provider       = iconfig.Provider
	Role           = iconfig.Role
	VerifyCfg      = iconfig.VerifyCfg
)

var (
	ErrNoProfileModel = iconfig.ErrNoProfileModel
	ProveidorPerNom   = iconfig.ProveidorPerNom
	ReadProfileModel  = iconfig.ReadProfileModel
)
