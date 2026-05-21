package web

import "embed"

//go:embed *.html *.js *.css *.json favicon.svg favicon.ico icons
var Files embed.FS
