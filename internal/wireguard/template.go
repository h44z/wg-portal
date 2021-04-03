package wireguard

import (
	"embed"
	"strings"
	"text/template"
)

//go:embed tpl/*
var Templates embed.FS

var templateCache *template.Template

func init() {
	var err error
	templateCache, err = template.New("server").Funcs(template.FuncMap{"StringsJoin": strings.Join}).ParseFS(Templates, "tpl/*.tpl")
	if err != nil {
		panic(err)
	}
}
