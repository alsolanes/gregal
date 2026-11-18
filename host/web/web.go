// Package web reexporta el servidor web de Gregal per a productes externs
// (vegeu gregal/host).
package web

import iweb "gregal/internal/web"

type (
	Hub     = iweb.Hub
	Plugin  = iweb.Plugin
	Product = iweb.Product
	Server  = iweb.Server
	User    = iweb.User
)

var (
	IsGuest         = iweb.IsGuest
	New             = iweb.New
	RegisterProduct = iweb.RegisterProduct
	UserFrom        = iweb.UserFrom
	WithUser        = iweb.WithUser
)
