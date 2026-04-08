package swagger

import "github.com/swaggo/swag"

const docTemplate = `{
    "swagger": "2.0",
    "info": {
        "title": "Go-Job API",
        "description": "Distributed job scheduling platform API",
        "version": "1.0"
    },
    "basePath": "/",
    "paths": {}
}`

type s struct{}

func (s *s) ReadDoc() string {
	return docTemplate
}

func init() {
	swag.Register(swag.Name, &s{})
}
