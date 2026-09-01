package http

import (
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

type Deps struct {
	Seal     portin.SealUseCase
	KeyRoots portin.KeyRootUseCase
}

func New(deps Deps) http.Handler {
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	config := huma.DefaultConfig("Sigryx API", "v1")
	config.DocsPath = ""
	config.CreateHooks = nil

	api := humagin.New(engine, config)
	registerHealthRoutes(api)

	if deps.Seal != nil {
		registerSealRoutes(api, deps.Seal)
	}

	if deps.KeyRoots != nil {
		registerKeyRootRoutes(api, deps.KeyRoots)
	}

	engine.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(scalarDocsHTML))
	})

	return engine
}

const scalarDocsHTML = `<!doctype html>
<html>
  <head>
    <title>Sigryx API</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script id="api-reference" data-url="/openapi.json"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`
