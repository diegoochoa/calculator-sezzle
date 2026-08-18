# Calculator

## Design rationale
A calculator built as two services in one repository: a React shell that composes expressions, and a Go API that evaluates them.

The project is intented to be a monorepo with frontend and backend, using security layers such as tokenization and rate limit on the endpoints. Docker compose on place to run the application without any other requirements, possibili

```
calculator-sezzle/
├── frontend/            React 19 + TypeScript + Vite
├── backend/             Go 1.23, REST/JSON, JWT, rate limited
├── docker-compose.yml   runs both, wired the way production is
└── PROMPTS.md           how this project was built, prompt by prompt
```

## Run it

Docker is the preferred way — it needs no Go toolchain and no Node, and it wires
the two halves the way production does: nginx serves the built SPA and proxies
`/v1` to the API, so the browser never makes a cross-origin request.

```bash
docker compose up --build
```

|              | URL                        |                                    |
| ------------ | -------------------------- | ---------------------------------- |
| **Frontend** | http://localhost:8080      | the whole product                  |
| API          | http://localhost:8081      | direct, for curl and other clients |
| API docs     | http://localhost:8081/docs | Swagger UI, no token needed        |

First time on macOS, or coming from a Docker Desktop install that left state
behind, see [backend/README.md](backend/README.md#first-time-on-macos) — there
are two leftovers that break every build with unhelpful errors.

## API documentation

The backend serves its own **Swagger UI** at
**http://localhost:8081/docs**, with the OpenAPI 3 specification at
[`/v1/openapi.yaml`](backend/api/openapi.yaml). Both are unauthenticated, since
a client has to read the documentation in order to learn how to authenticate.

Every endpoint is callable from the page: run `POST /v1/auth/token` first, paste
the token into **Authorize**, then try the rest.

The specification is hand-written and kept honest by a test that fails the build
if it drifts from the routes actually served — in either direction — and that
checks every `$ref` resolves. Details in
[backend/README.md](backend/README.md#api-documentation).

```bash
# generate a typed client from it
npx @openapitools/openapi-generator-cli generate \
  -i http://localhost:8081/v1/openapi.yaml -g typescript-fetch -o ./client
```

⚠️ `/docs` loads Swagger UI's assets from a pinned CDN build, so that page needs
network access. The specification itself is embedded in the binary, so offline
tooling and client generation are unaffected.

## Develop it

Each side keeps its own toolchain, its own tests and its own README. They do not
interfere.

```bash
cd backend  && make run     # :8080, then make test / make cover / make fuzz
cd frontend && npm run dev  # :5173, proxying /v1 to the API above
```

|          | Docs                                     | Tests            | Coverage                                     |
| -------- | ---------------------------------------- | ---------------- | -------------------------------------------- |
| Frontend | [frontend/README.md](frontend/README.md) | `npm test` — 97 | `npm run test:coverage` — ~98%, gated at 85% |
| Backend  | [backend/README.md](backend/README.md)   | `make test`      | `make cover` — 89.7%, gated at 85%           |

## Coverage reports

**[COVERAGE.md](COVERAGE.md)** has the full breakdown, per package and per
metric. The browsable HTML reports are committed so they can be read without
cloning:

| | Report | Current |
|---|---|---|
| Frontend | [`frontend/coverage/index.html`](frontend/coverage/index.html) | **97.7%** statements · 95.8% branches · 97.7% functions · 99.4% lines |
| Backend | [`backend/coverage.html`](backend/coverage.html) | **89.7%** statements |

> GitHub serves committed `.html` as source rather than rendering it. To read a
> report in a browser without cloning, prefix its URL with
> `https://htmlpreview.github.io/?`. [COVERAGE.md](COVERAGE.md) renders natively
> either way.

Both are gated at **85%**, and the gates fail the build rather than warn:

```bash
make coverage        # both suites, both thresholds enforced
make coverage-html   # regenerate both reports, printing their paths
```

What each side excludes from measurement, and why, is in
[backend/README.md](backend/README.md#testing) and
[frontend/README.md](frontend/README.md#testing) — briefly: `cmd/` and barrels
are wiring, and wiring is kept honest by being thin, not by being tested.

## How the two fit together

The Go service owns **all** arithmetic. The frontend holds no engine at all: its
keys append to an expression string, `=` posts it, and the server answers with
both a number and the formatted display string. One set of rounding rules, one
set of domain checks, one place for them to be wrong.

That has consequences worth knowing before reading either README:

- **Operator precedence is real.** `2 + 3 × 4` is 14. The frontend's original
  engine was strictly left to right and answered 20.
- **`%` divides by one hundred.** `50%` is `0.5`. The old UI's relative percent,
  where `200 + 10%` was 220, was a display convention rather than a grammar rule.
- **The keypad cannot build an expression the grammar rejects.** Pressing `2`
  then `√` writes `2 × √` — the operator visible, never implied — because the
  grammar deliberately has no implicit multiplication to guess with. The `)`
  key is likewise disabled whenever no bracket is open.

**Scope.** The calculator does the four operations plus brackets, percent,
square root and exponentiation. An earlier iteration carried a scientific keypad
— trigonometry, logarithms, hyperbolics, 32 functions in all — and it was
removed deliberately: the brief prioritises correctness, clarity and
maintainability over extra features, and every one of those operations was
surface area to read and a judgement call to defend. `PROMPTS.md` records how
that decision was reached.

## Security note

The frontend's client credentials ship inside the JavaScript bundle and are
readable by any user. That is inherent to a browser SPA, not an oversight: the
token buys per-client attribution and keeps non-browser consumers out, while the
real defence is the API's rate limiting, CORS allowlist and payload caps. Never
put a production secret in a `VITE_` variable.
