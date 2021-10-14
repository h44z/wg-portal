package main

import "embed"

//go:embed assets/tpl/*
var Templates embed.FS

//go:embed assets/css/*
//go:embed assets/fonts/*
//go:embed assets/img/*
//go:embed assets/js/*
var Statics embed.FS
