package core

import "embed"

//go:embed assets/tpl/*
var apiTemplates embed.FS

//go:embed assets/css/*
//go:embed assets/fonts/*
//go:embed assets/img/*
//go:embed assets/js/*
//go:embed assets/doc/*
var apiStatics embed.FS

//go:embed frontend-dist/assets/*
//go:embed frontend-dist/img/*
//go:embed frontend-dist/index.html
//go:embed frontend-dist/favicon.ico
//go:embed frontend-dist/favicon.png
//go:embed frontend-dist/favicon-large.png
var frontendStatics embed.FS
