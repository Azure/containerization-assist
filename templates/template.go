package templates

import "embed"

//go:embed all:*
var Templates embed.FS // templates includes pre-rendered templates from github.com/Azure/draft
