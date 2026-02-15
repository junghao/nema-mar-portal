package main

import (
	"net/http"

	"github.com/GeoNet/kit/weft"
)

var mux *http.ServeMux

func init() {
	mux = http.NewServeMux()

	mux.HandleFunc("/", weft.MakeHandler(weft.NoMatch, weft.TextError))
	mux.HandleFunc("/soh/up", weft.MakeHandler(weft.Up, weft.TextError))
	mux.HandleFunc("/soh", weft.MakeHandler(weft.Soh, weft.TextError))

	// Editor portal (HTML pages with nonce for inline JS)
	mux.HandleFunc("/gha-portal", weft.MakeHandlerWithNonce(portalPageHandler, weft.HTMLError))
	mux.HandleFunc("/gha-portal/preview", weft.MakeHandlerWithNonce(portalPreviewHandler, weft.HTMLError))

	// Dashboard (HTML page with nonce for map embed JS)
	mux.HandleFunc("/dashboard", weft.MakeHandlerWithNonce(dashboardHandler, weft.HTMLError))

	// App's own JSON API endpoints (called by JS on editor page).
	// These are NOT FastSchema proxy endpoints — they are nema-mar-app's own
	// endpoints that internally call FastSchema for data persistence.
	mux.HandleFunc("/api/events", weft.MakeHandler(apiEventsHandler, weft.TextError))
	mux.HandleFunc("/api/eat", weft.MakeHandler(apiEATHandler, weft.TextError))
	mux.HandleFunc("/api/publish", weft.MakeHandler(apiPublishHandler, weft.TextError))
	mux.HandleFunc("/api/upload", weft.MakeDirectHandler(apiUploadHandler, weft.TextError))
}
