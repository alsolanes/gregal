// Package profile reexporta el perfil de producte de Gregal per a productes
// externs (vegeu gregal/host).
package profile

import iprofile "gregal/internal/profile"

var (
	ConfigDirFor      = iprofile.ConfigDirFor
	DefaultDataDirFor = iprofile.DefaultDataDirFor
	Product           = iprofile.Product
	SetProduct        = iprofile.SetProduct
)
