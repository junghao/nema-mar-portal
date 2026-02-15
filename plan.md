# NEMA-MAR-Portal Implementation Plan

## Context

NGMC needs a service to create Emergency Advisory Texts (EAT) for earthquake/tsunami events and distribute them to NEMA stakeholders via email and a web dashboard. The service uses **FastSchema** (a Go-based headless CMS) as a sidecar for content storage, backed by PostgreSQL and S3. The Go application (**nema-mar-app**) handles all HTTP requests and uses FastSchema purely as an internal data store — it does **not** proxy or expose FastSchema's API to external users. All external-facing endpoints are nema-mar-app's own HTML pages and JSON APIs that internally call FastSchema for persistence.

---

## Architecture Overview

```
                         Internet
                            |
                     [AWS ALB :443/:80]
                            |
                    +-----------------+
                    |   ECS Task      |
                    |                 |
                    | +-------------+ |
  HTTP :8080 <------| nema-mar-app | |
                    | +------+------+ |
                    |        |        |
                    |  localhost:8000  |
                    |        |        |
                    | +------+------+ |
                    | | FastSchema  | |
                    | | (sidecar)   | |
                    | +------+------+ |
                    +----|-------|-----+
                         |       |
                  [PostgreSQL]  [S3 Bucket]
                   (RDS)     (object store)
                                        + [SMTP Server]
```

---

## 1. Project Structure

```
nema-mar-portal/
├── .github/workflows/
│   └── dev.yml                    # CI: build, test, schema check, Docker
├── cmd/nema-mar-app/
│   ├── server.go                  # main(), HTTP server, health check
│   ├── routes.go                  # init() mux, route registration
│   ├── log.go                     # Logger + DataDog metrics
│   ├── portal_handler.go          # /gha-portal editor handlers
│   ├── dashboard_handler.go       # /dashboard read-only handlers
│   ├── api_handler.go             # /api/* JSON endpoints (events, publish, upload)
│   ├── templates.go               # Template loading, Page struct, funcMap
│   ├── templates/
│   │   ├── base.html              # Shared layout
│   │   ├── editor.html            # EAT editor form
│   │   ├── dashboard.html         # Dashboard display
│   │   └── preview.html           # Preview (dashboard-like, unsaved data)
│   ├── portal_handler_test.go
│   ├── dashboard_handler_test.go
│   ├── api_handler_test.go
│   └── routes_test.go
├── internal/
│   ├── fastschema/
│   │   ├── client.go              # HTTP client for FastSchema sidecar
│   │   ├── client_test.go
│   │   └── types.go               # EAT struct, File struct, API response types
│   ├── valid/
│   │   ├── valid.go               # Query validators (weft pattern)
│   │   └── valid_test.go
│   ├── pdf/
│   │   ├── generate.go            # PDF generation using go-pdf/fpdf
│   │   └── generate_test.go
│   └── email/
│       ├── smtp.go                # SMTP email with PDF attachment
│       └── smtp_test.go
├── schema/
│   └── eat.json                   # FastSchema JSON schema definition
├── terraform/
│   ├── main.tf                    # ECS task def (sidecar), RDS Postgres, S3
│   ├── variables.tf
│   └── outputs.tf
├── Dockerfile                     # Multi-stage build for nema-mar-app
├── go.mod
└── go.sum
```

**Key references to follow:**
- [server.go](../gloria/cmd/gloria-ws/server.go) - main() pattern, health check, env vars
- [routes.go](../gloria/cmd/gloria-ws/routes.go) - mux init() pattern
- [kit/weft/handlers.go](../kit/weft/handlers.go) - RequestHandler, RequestHandlerWithNonce, MakeHandler, MakeHandlerWithNonce

---

## 2. FastSchema Schema (`schema/eat.json`)

```json
{
  "name": "eat",
  "namespace": "eats",
  "label_field": "event_title",
  "fields": [
    { "name": "event_title", "type": "string", "label": "Event Title", "sortable": true, "filterable": true },
    { "name": "location", "type": "string", "label": "Location" },
    { "name": "event_date", "type": "time", "label": "Event Date (UTC)", "sortable": true, "filterable": true },
    { "name": "magnitude", "type": "float32", "label": "Magnitude" },
    { "name": "earthquake_url", "type": "string", "label": "Earthquake URL", "optional": true },
    { "name": "version", "type": "int", "label": "Version", "sortable": true, "default": 1 },
    { "name": "event_comments", "type": "text", "label": "Event Comments", "optional": true },
    { "name": "beach_marine_threat", "type": "bool", "label": "Beach and Marine Threat", "default": false },
    { "name": "land_threat", "type": "bool", "label": "Land Threat", "default": false },
    { "name": "status", "type": "enum", "label": "Status", "enums": [{"label":"Preliminary","value":"preliminary"},{"label":"Confirmed","value":"confirmed"}], "default": "preliminary" },
    { "name": "tep_activated", "type": "bool", "label": "TEP Activated", "default": false },
    { "name": "attachments", "type": "file", "label": "Attachments", "multiple": true, "optional": true }
  ]
}
```

Event uniqueness: `event_title` + `version`. The `event_title` is computed as `M{magnitude}-{location}-{date}`.

---

## 3. Go Types (`internal/fastschema/types.go`)

```go
type EAT struct {
    ID                int       `json:"id"`
    EventTitle        string    `json:"event_title"`
    Location          string    `json:"location"`
    EventDate         time.Time `json:"event_date"`
    Magnitude         float32   `json:"magnitude"`
    EarthquakeURL     string    `json:"earthquake_url"`
    Version           int       `json:"version"`
    EventComments     string    `json:"event_comments"`
    BeachMarineThreat bool      `json:"beach_marine_threat"`
    LandThreat        bool      `json:"land_threat"`
    Status            string    `json:"status"` // "preliminary" | "confirmed"
    TEPActivated      bool      `json:"tep_activated"`
    Attachments       []File    `json:"attachments,omitempty"`
    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}

type File struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
    Path string `json:"path"`
    URL  string `json:"url,omitempty"`
    Type string `json:"type,omitempty"`
}
```

---

## 4. FastSchema Client (`internal/fastschema/client.go`)

Internal HTTP client wrapping all FastSchema sidecar communication at `http://localhost:8000`. This is used exclusively by nema-mar-app's own handlers — FastSchema is never exposed externally.

| Method | HTTP Call | Purpose |
|--------|-----------|---------|
| `Login(user, pass)` | `POST /api/auth/login` | Get JWT token at startup |
| `ListEATs(since time.Time)` | `GET /api/content/eat?filter=...&sort=-created_at` | List EATs within date range |
| `GetEAT(id int)` | `GET /api/content/eat/{id}` | Get single EAT by ID |
| `GetLatestVersion(eventTitle)` | `GET /api/content/eat?filter=...&sort=-version&limit=1` | Latest version for an event |
| `ListDistinctEvents(days int)` | List + deduplicate in Go | Distinct event titles |
| `CreateEAT(eat)` | `POST /api/content/eat` | Create new EAT record |
| `UploadFile(name, data)` | `POST /api/file/upload` | Upload attachment to S3 |
| `ApplySchema(json)` | `POST /api/schema` | Apply/update schema at startup |

Authentication: JWT token from `Login()`, stored in client, sent as `Authorization: Bearer {token}` header.

---

## 5. Route Definitions (`cmd/nema-mar-app/routes.go`)

```go
mux.HandleFunc("/", weft.MakeHandler(weft.NoMatch, weft.TextError))
mux.HandleFunc("/soh/up", weft.MakeHandler(weft.Up, weft.TextError))
mux.HandleFunc("/soh", weft.MakeHandler(weft.Soh, weft.TextError))

// Editor portal (HTML, needs nonce for inline JS)
mux.HandleFunc("/gha-portal", weft.MakeHandlerWithNonce(portalPageHandler, weft.HTMLError))
mux.HandleFunc("/gha-portal/preview", weft.MakeHandlerWithNonce(portalPreviewHandler, weft.HTMLError))

// Dashboard (HTML, needs nonce for map embed JS)
mux.HandleFunc("/dashboard", weft.MakeHandlerWithNonce(dashboardHandler, weft.HTMLError))

// App's own JSON API endpoints (called by JS on editor page)
// These are NOT FastSchema proxy endpoints — they are nema-mar-app's own
// endpoints that internally call FastSchema for data persistence.
mux.HandleFunc("/api/events", weft.MakeHandler(apiEventsHandler, weft.TextError))
mux.HandleFunc("/api/eat", weft.MakeHandler(apiEATHandler, weft.TextError))
mux.HandleFunc("/api/publish", weft.MakeHandler(apiPublishHandler, weft.TextError))
mux.HandleFunc("/api/upload", weft.MakeDirectHandler(apiUploadHandler, weft.TextError))
```

### Handler Descriptions

| Handler | Method | Description |
|---------|--------|-------------|
| `portalPageHandler` | GET/POST | Renders editor page. GET: event dropdown (last 7 days). POST: form submission. |
| `portalPreviewHandler` | POST | Renders dashboard-like preview from unsaved form data |
| `dashboardHandler` | GET | Renders read-only dashboard. `?event_title=X&version=Y` or shows latest |
| `apiEventsHandler` | GET | Returns JSON list of distinct event titles from last 7 days (internally queries FastSchema) |
| `apiEATHandler` | GET | Returns JSON of specific EAT (`?id=N` or `?event_title=X`) (internally queries FastSchema) |
| `apiPublishHandler` | POST | Validates input, saves EAT to FastSchema, generates PDF, sends email. Returns JSON |
| `apiUploadHandler` | POST | Accepts file upload, stores via FastSchema's file API internally, returns file reference |

### Publish Flow

1. Parse JSON body -> validate all fields
2. If new event: compute `event_title`, set `version = 1`
3. If new version: lookup latest version, set `version = latest + 1`
4. Upload any new attachments via `fsClient.UploadFile()`
5. `fsClient.CreateEAT(eat)` to persist
6. `pdf.GenerateEATPDF(eat)` -> PDF bytes
7. `email.SendEATEmail(config, eat, pdfBytes)` -> SMTP delivery
8. Return `{"success": true, "id": N, "version": V}`

---

## 6. HTML Templates

All using Go `html/template`. Minimal styling (functionality only as per requirements).

### `base.html`
- `<head>` with charset, title block
- `<nav>` with links to `/gha-portal` and `/dashboard`
- `{{block "content" .}}` and `{{block "scripts" .}}`

### `editor.html` (`/gha-portal`)
- Event selector: `<select>` dropdown populated from API + "Create New Event" button with confirm popup
- Form fields: location, event_date (datetime-local), magnitude (number), earthquake_url, event_comments (textarea), beach_marine_threat/land_threat/tep_activated (checkboxes), status (select)
- Auto-title: `<span id="event-title">` updated by JS `oninput` (new event mode only)
- Protected fields: `readonly` attribute on title-related fields in new version mode
- Attachments: `<input type="file" multiple>` with drag-drop zone (JS)
- Buttons: "Preview" (POST to `/gha-portal/preview`), "Publish" (AJAX POST to `/api/publish`)
- All inline JS uses nonce: `<script nonce="{{.Nonce}}">`

### `dashboard.html` (`/dashboard`)
- Banner div (conditionally shown) for new event/version notification
- `<dl>` definition list for all EAT fields
- Attachments: `<img>` for images, `<a>` for PDFs, `<pre>` for text
- Map: `<iframe>` embedding GeoNet map with epicenter
- Version history table for all versions of this event

### `preview.html` (`/gha-portal/preview`)
- Same layout as dashboard but rendered from form data (not persisted)

---

## 7. PDF Generation (`internal/pdf/generate.go`)

Uses `github.com/go-pdf/fpdf`:
- A4 portrait, 15mm margins
- Header: "Emergency Advisory Text" title
- Event title, version, status
- All field values as label:value pairs
- Multiline event comments
- Attachment list (names + types; images embedded if available)
- Returns `[]byte` (in-memory, no temp files)

---

## 8. Email Service (`internal/email/smtp.go`)

- Config from env vars: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`, `SMTP_RECIPIENTS`
- MIME multipart message: text/plain body (EAT summary) + application/pdf attachment
- Subject: `EAT: {event_title} (Version {N}) - {Status}`
- Uses `net/smtp` with STARTTLS

---

## 9. Terraform (`terraform/`)

### Resources

| Resource | Purpose |
|----------|---------|
| `aws_db_instance.fastschema_db` | PostgreSQL RDS for FastSchema (engine=postgres, driver=pgx) |
| S3 bucket/prefix | FastSchema object store at `nema-mar/files` |
| `aws_ecs_task_definition.nema_mar_portal` | **Two containers**: nema-mar-app (:8080) + fastschema sidecar (:8000) |
| `aws_ecs_service.nema_mar_portal` | ECS service with ALB target group |
| ALB + target group + listeners | HTTP/HTTPS load balancer pointing to nema-mar-app:8080 |
| CloudWatch log groups | Separate logs for app and fastschema |
| IAM roles | Task role (S3 access) + execution role (SSM params, ECR pull, CloudWatch) |
| SSM parameters | SMTP creds, FastSchema admin creds |

### Sidecar Pattern (Key Design)
- FastSchema container has **empty portMappings** (not exposed to host/ALB)
- nema-mar-app uses `dependsOn: [{containerName: "fastschema", condition: "HEALTHY"}]`
- Both share `localhost` network within the ECS task (bridge mode)

---

## 10. Dockerfile

Multi-stage build following the gloria Dockerfile pattern:
- **Builder**: `ghcr.io/geonet/base-images/golang:1.23-alpine3.21`, `GOFLAGS=-mod=vendor`, `CGO_ENABLED=0`
- **Runner**: `ghcr.io/geonet/base-images/alpine:3.21`
- Copy binary, templates dir, schema dir to `/app/`
- Run as `nobody` user

FastSchema uses its own Docker image (`ghcr.io/fastschema/fastschema:latest`), configured via env vars in the ECS task definition.

---

## 11. GitHub Actions CI (`.github/workflows/dev.yml`)

Jobs:
1. **build-app**: Reusable `GeoNet/Actions/.github/workflows/reusable-go-apps.yml` (Go build + test)
2. **schema-check**: Spin up PostgreSQL service container + FastSchema Docker container, apply `schema/eat.json`, verify schema creation succeeds
3. **build-images**: Reusable Docker build workflow, push to ECR

The schema-check job catches schema definition errors before deployment.

---

## 12. Testing Strategy

| Test File | What It Tests |
|-----------|---------------|
| `routes_test.go` | Smoke tests: all routes return expected status using `wefttest.Requests` |
| `portal_handler_test.go` | GET returns 200 HTML; POST with valid/invalid form data; method not allowed |
| `dashboard_handler_test.go` | GET returns 200 HTML; query param filtering |
| `api_handler_test.go` | GET `/api/events` returns JSON; POST `/api/publish` success/validation error |
| `internal/fastschema/client_test.go` | Mock HTTP server simulating FastSchema; CRUD operations; error handling |
| `internal/valid/valid_test.go` | Table-driven validation: event_id, days, magnitude, location |
| `internal/pdf/generate_test.go` | Generate PDF from sample EAT; verify PDF header bytes |
| `internal/email/smtp_test.go` | MIME message construction; mock SMTP server |

Test infrastructure: `httptest.NewServer` with mock FastSchema responses; `httptest.NewServer(mux)` for handler tests.

---

## 13. Security Considerations

- **CSP**: Use `weft.MakeHandlerWithNonce` for all HTML pages (nonce for inline scripts)
- **CSRF**: Generate CSRF token per session, validate on POST handlers
- **Input validation**: All user input validated in `internal/valid/`; sanitize before template rendering
- **FastSchema isolation**: Not port-mapped to host; only reachable via localhost within ECS task
- **Secrets**: SMTP creds and FS admin creds via SSM Parameter Store (not env vars in plain text)
- **Security headers**: Handled by weft (X-Frame-Options, HSTS, X-Content-Type-Options, etc.)

---

## 14. Publish Data Flow Diagram

```
Browser                 nema-mar-app            FastSchema (localhost:8000)
  |                         |                           |
  |-- POST /api/publish --->|                           |
  |   {EAT JSON + files}   |                           |
  |                         |-- POST /api/file/upload -->|
  |                         |   (per attachment)         |
  |                         |<-- {file_id, url} ---------|
  |                         |                           |
  |                         |-- POST /api/content/eat -->|
  |                         |   {EAT data + file refs}   |
  |                         |<-- {id, created_at} -------|
  |                         |                           |
  |                         |-- GenerateEATPDF() -----> (in-memory)
  |                         |-- SendEATEmail() -------> [SMTP]
  |                         |                           |
  |<-- {success, id, ver} --|                           |
```

---

## 15. Implementation Sequence

1. **Foundation**: `go.mod`, types, valid, server.go, routes.go, log.go (health check endpoints work)
2. **FastSchema Client**: client.go with CRUD methods + tests with mock server
3. **Schema**: `schema/eat.json` definition
4. **Dashboard**: dashboard_handler.go + templates (read-only, simplest path first)
5. **Editor Portal**: portal_handler.go + editor template (new event/new version logic)
6. **Publish Flow**: api_handler.go + pdf/generate.go + email/smtp.go (full pipeline)
7. **Preview**: preview handler + template
8. **Infrastructure**: Dockerfile, terraform/, GitHub Actions CI
9. **Polish**: CSRF tokens, input sanitization, dashboard notification banner, comprehensive tests

---

## Verification

1. **Unit tests**: `go test ./...` passes for all packages
2. **Local smoke test**: Run `go run ./cmd/nema-mar-app` with a local FastSchema Docker container + PostgreSQL; verify `/soh`, `/gha-portal`, `/dashboard` respond correctly
3. **Docker build**: `docker build -t nema-mar-app .` succeeds
4. **Schema check**: Apply `schema/eat.json` to a fresh FastSchema instance; verify CRUD operations work
5. **PDF generation**: Unit test verifies valid PDF bytes output
6. **Email**: Unit test verifies MIME message construction
