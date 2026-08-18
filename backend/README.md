# Calculator API

REST/JSON calculation service in Go. It parses and evaluates mathematical
expressions with full operator precedence, validates input, authenticates
clients with JWTs and rate limits them.

Companion to the React frontend in [`../frontend`](../frontend), which is a thin
shell over this service — it composes expressions, everything here evaluates
them.

## Running it — Docker is the preferred way

Compose brings up the API and the frontend together, wired the way production
is: nginx serves the built SPA and proxies `/v1` to the API, so the browser
never makes a cross-origin request and CORS never enters the picture. Nothing
but a container runtime is required — no Go toolchain, no Node.

Compose lives at the repository root, one level up:

```bash
cd .. && docker compose up --build
```

| | URL | |
|---|---|---|
| Frontend | http://localhost:8080 | the whole product |
| API | http://localhost:8081 | direct, for curl and other clients |

```bash
docker compose ps            # health of both containers
docker compose logs -f api   # structured request logs
docker compose down          # stop and remove
```

The API image is `distroless/static` at **15.4 MB**, running as `nonroot` with
no shell in it at all — `docker compose exec api /bin/sh` fails by design, so an
attacker reaching RCE has nothing to pivot with. That is also why the container
healthcheck is `/server -healthcheck`: there is no `curl` or `wget` to call, so
the binary probes itself.

### First time on macOS

Docker Desktop is not required. Colima provides the daemon:

```bash
brew install colima docker docker-compose docker-buildx
mkdir -p ~/.docker/cli-plugins
ln -sfn /opt/homebrew/lib/docker/cli-plugins/docker-compose ~/.docker/cli-plugins/
ln -sfn /opt/homebrew/lib/docker/cli-plugins/docker-buildx  ~/.docker/cli-plugins/
colima start --cpu 4 --memory 6 --disk 40
```

⚠️ **If Docker Desktop was ever installed on this machine**, two leftovers will
break every build with confusing errors. Both are safe to clear:

```bash
# "open ~/.docker/buildx/current: permission denied" — the file is root-owned
rm -f ~/.docker/buildx/current

# "error getting credentials — exec: docker-credential-desktop: not found"
# remove the "credsStore": "desktop" entry from ~/.docker/config.json
```

`colima stop` shuts the VM down when you are finished.

## Running it without containers

Needs Go 1.23+. Useful for a fast edit-run loop and for the test targets, which
are not containerised.

```bash
make run     # :8080 with development defaults
make test    # full suite with the race detector
make cover   # coverage, gated at 85%
make fuzz    # parser fuzzing
make help    # every target
```

## Quick start

Against either of the above — `:8081` under Compose, `:8080` under `make run`:

```bash
API=http://localhost:8081

TOKEN=$(curl -s $API/v1/auth/token \
  -d '{"clientId":"web","clientSecret":"dev-secret"}' | jq -r .token)

curl -s $API/v1/calculate -H "Authorization: Bearer $TOKEN" \
  -d '{"expression":"2 + 3 * 4"}'
# {"result":14,"formatted":"14","expression":"2 + 3 * 4"}
```

## API documentation

Swagger UI is served by the API itself:

| | |
|---|---|
| **Browsable docs** | http://localhost:8081/docs |
| Raw specification | http://localhost:8081/v1/openapi.yaml |

Both are unauthenticated — a client has to read the documentation in order to
learn how to authenticate. "Try it out" works from the page: call
`POST /v1/auth/token` first, then paste the token into **Authorize**.

The specification is hand-written at [`api/openapi.yaml`](api/openapi.yaml)
rather than generated from annotations, which would mean a code generator, a
build step and a `docs` package in the tree for an API this size. What keeps it
honest is [`openapi_test.go`](internal/httpapi/openapi_test.go), which fails the
build when the document and the server disagree:

- every documented route is actually served, and every served route is
  documented — drift in either direction breaks the build
- every `$ref` resolves (49 of them), so the page cannot render with a
  "could not resolve reference"
- every error code the engine can raise appears in the enum clients switch on
- the documentation stays reachable without a token

⚠️ Swagger UI's assets come from a pinned CDN build, so **`/docs` needs network
access**. That is deliberate: vendoring them would add several megabytes to a
15 MB image whose point is having nothing spare in it. The specification itself
is embedded in the binary, so offline tooling and client generation are
unaffected.

Generate a client from it with whatever you already use:

```bash
npx @openapitools/openapi-generator-cli generate \
  -i http://localhost:8081/v1/openapi.yaml -g typescript-fetch -o ./client
```

## Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| `GET` | `/docs` | none | Swagger UI |
| `GET` | `/v1/openapi.yaml` | none | The specification |
| `POST` | `/v1/auth/token` | client credentials | Exchange id/secret for a JWT |
| `POST` | `/v1/calculate` | Bearer | Evaluate one expression |
| `POST` | `/v1/calculate/batch` | Bearer | Evaluate many; per-item success or failure |
| `POST` | `/v1/validate` | Bearer | Parse only — syntax check, no evaluation |
| `GET` | `/v1/functions` | Bearer | Catalog: functions, operators, limits |
| `GET` | `/healthz` | none | Liveness |
| `GET` | `/readyz` | none | Readiness |

### `POST /v1/calculate`

```jsonc
// request — precision (1-15, default 12) is optional
{ "expression": "2 + 3 * 4", "precision": 12 }

// 200
{ "result": 14, "formatted": "14", "expression": "2 + 3 * 4" }
```

`result` is a JSON number, `formatted` the display string — the client should
not have to re-derive the engine's rounding rules.

### `POST /v1/calculate/batch`

Returns **200 even when individual expressions fail**. A partial failure is the
expected outcome, and one bad expression should not discard the results that
succeeded.

```jsonc
{ "expressions": ["1+1", "1/0"] }

{ "results": [
    { "expression": "1+1", "result": 2, "formatted": "2" },
    { "expression": "1/0", "error": { "code": "DIVISION_BY_ZERO", "message": "Can't divide by zero", "position": 1 } }
  ] }
```

### `POST /v1/validate`

Parse-only, so a UI can check syntax on every keystroke without triggering a
division by zero or a slow evaluation. An invalid expression is a **200 with
`valid: false`**, not a 422: the client asked whether the text parses, and "no"
is a successful answer to that question.

### Errors

Every failure — 400, 401, 404, 405, 413, 422, 429, 500 — uses one envelope:

```json
{
  "error": { "code": "DIVISION_BY_ZERO", "message": "Can't divide by zero", "position": 7 },
  "requestId": "8bdcaaea2543f569c52c9347"
}
```

`position` is the 0-based rune offset in the submitted expression, so a client
can underline exactly what broke. `requestId` is echoed in the `X-Request-Id`
header and stamped on every log line for that request.

| Code | Status | Raised by |
|---|---|---|
| `INVALID_REQUEST` | 400 | Malformed JSON, wrong types, unknown fields, out-of-range `precision` |
| `UNAUTHORIZED` | 401 | Missing, malformed, expired or forged token; bad client credentials |
| `NOT_FOUND` / `METHOD_NOT_ALLOWED` | 404 / 405 | Routing |
| `PAYLOAD_TOO_LARGE` | 413 | Body over `CALC_MAX_BODY_BYTES` |
| `EMPTY_EXPRESSION` | 422 | Empty or whitespace-only expression |
| `SYNTAX_ERROR` | 422 | `2 +`, `2 3`, stray characters |
| `UNBALANCED_PAREN` | 422 | `((1+2)`, `()`, `1+2)` |
| `UNKNOWN_IDENTIFIER` | 422 | `foo`, `foo(1)` |
| `WRONG_ARITY` | 422 | `log(8)`, `sin(1,2)` |
| `EXPRESSION_TOO_LONG` / `DEPTH_EXCEEDED` | 422 | Over the length or nesting limit |
| `DIVISION_BY_ZERO` | 422 | `1/0`, `1 mod 0`, `1 div 0`, `0^-1` |
| `DOMAIN_ERROR` | 422 | `sqrt(-1)`, `ln(0)`, `asin(2)`, `fact(-1)`, `(-8)^0.5` |
| `UNDEFINED_RESULT` | 422 | `tan(90)` in degrees |
| `OVERFLOW` | 422 | `exp(1000)`, `fact(171)`, `1e400` |
| `RATE_LIMITED` | 429 | Over the configured rate |
| `TIMEOUT` / `INTERNAL_ERROR` | 503 / 500 | Request deadline, recovered panic |

400 versus 422 is the line between "the request was malformed" and "the request
was understood but cannot be computed".

## Grammar

| Precedence | Operators | Associativity |
|---|---|---|
| 1 | `+` `-` | left |
| 2 | `*` `/` | left |
| 3 | unary `-` `+` `√` | prefix |
| 4 | `^` | **right** — `2^3^2` is 512 |
| 5 | `%` | postfix |

`-2^2` is `-4`: negation binds looser than exponentiation. `2^-3` works without
parentheses.

**Functions** — `sqrt(x)` and `abs(x)`.

**Unicode aliases**, because the UI renders these glyphs on its keys: `×` `·`
for `*`, `÷` for `/`, `−` `–` for `-`, and `√`.

The function set is deliberately small. Trigonometry, logarithms, hyperbolics,
factorials and constants were removed along with the scientific keypad: the
brief prioritises correctness, clarity and maintainability over extra features,
and each of those brought a judgement call — angle units, domain edges, argument
separators — that the requirements never needed answered. The registry and its
arity checking are untouched, so adding one back is a single map entry plus its
catalog text.

Two deliberate choices remain:

- **`%` is postfix divide-by-100**, so `50%` is `0.5`, and `200 * 10%` is `20`.
- **No implicit multiplication.** `2 3` is a syntax error; write `2 * 3`.
  Guessing here is a well-known source of wrong answers — `1/2x` has two
  defensible readings and both are wrong for somebody. The keypad enforces this
  by writing the `×` itself rather than leaving it implied.

`GET /v1/functions` serves all of this as JSON, so a client can build its keypad
and syntax hints from the server's real capability rather than a hand-maintained
duplicate.

## Architecture

```
cmd/server/main.go          # config → dependencies → router → server → drain
internal/
├── calc/                   # the engine — pure Go, imports no net/http
│   ├── number.go           # precision-safe arithmetic
│   ├── lexer.go            # source → tokens (rune offsets)
│   ├── parser.go           # precedence climbing → AST
│   ├── eval.go             # AST → float64
│   ├── functions.go        # function registry with arity and domain guards
│   ├── catalog.go          # the grammar, as data
│   └── errors.go           # CalcError{Code, Message, Position}
├── httpapi/                # handlers, decoding, status mapping, routing
├── middleware/             # recover, request id, logging, CORS, body limit, auth, rate limit
├── auth/                   # JWT issue/verify, client credential store
├── config/                 # environment parsing with boot-time validation
└── httpx/                  # response envelope + request context, shared by the two layers above
```

`internal/calc` never imports `net/http`, and the HTTP layer never does
arithmetic. That boundary is what makes the engine exhaustively table-testable
and reusable outside this server.

Middleware order is deliberate: **recover → request id → logging → CORS → body
limit → auth → rate limit → handler**. Recovery is outermost so a panic in any
layer still produces the JSON envelope; authentication precedes rate limiting so
the limiter keys on the authenticated client rather than a shared NAT address.

No web framework — Go 1.22+ `ServeMux` handles method-and-path routing. The only
dependencies are `golang-jwt/jwt/v5`, `golang.org/x/time/rate` and
`golang.org/x/crypto/bcrypt`.

## Precision

The engine scales operands to integers by a power of ten and works there
whenever that fits inside `MaxSafeInteger`, so `0.1 + 0.2` is `0.3`. Otherwise
it rounds to 12 significant digits. **Integers are never rounded**, so exact
results such as `2^53` survive in full.

`sqrt` and non-integer powers cannot use that trick and are rounded to 12
significant digits instead, so `2^0.5` is `1.41421356237` rather than a value
carrying float noise in its last places.

`NaN` and `±Inf` are converted to typed errors **before** JSON encoding —
neither is representable in JSON and both would otherwise surface as a 500.

⚠️ This is display-grade arithmetic, not ledger arithmetic. When the financial
operations land (TVM, APR/APY, installment splits), that is the moment to move
to a decimal type such as `shopspring/decimal`.

## Security

`POST /v1/auth/token` takes `{clientId, clientSecret}`, compares against bcrypt
hashes, and returns an HS256 JWT (`sub`, `iss`, `iat`, `nbf`, `exp`, `jti`).
Middleware pins the algorithm — rejecting `alg: none` and any asymmetric
algorithm, the classic JWT break — and verifies signature, expiry and issuer.

Deliberate choices:

- **Every rejection reads identically.** Distinguishing "no such client" from
  "wrong secret", or "expired" from "bad signature", turns the error message
  into an oracle. Unknown client ids are still run through a bcrypt comparison
  against a decoy hash so the timing does not leak either.
- **The token route has its own, tighter limiter**, keyed by source address.
  bcrypt makes it the expensive path and the obvious brute-force target.
- **The server refuses to start** without a `CALC_JWT_SECRET` of at least 32
  bytes in production, with an example secret in production, with plaintext
  client secrets in production, or with `CORS_ORIGINS=*` in production.
- **`X-Forwarded-For` is honoured only from `CALC_TRUSTED_PROXIES`.** Trusting
  it unconditionally lets any client forge the header and walk past the per-IP
  limit.

⚠️ **A browser SPA cannot hold a secret.** Anything shipped in JavaScript is
readable by any user, so the frontend's client secret is not confidential. The
JWT provides per-client attribution and gates non-browser consumers; the actual
defence for the public surface is the rate limiter, the CORS allowlist and the
body and expression caps.

## Rate limiting

Token bucket, keyed by JWT subject on the calculation routes and by source
address on the token route. `X-RateLimit-Limit` and `X-RateLimit-Remaining` on
every response; `Retry-After` on a 429.

The bucket map is swept on a TTL. A map that only grows is the standard bug in
this pattern — with per-IP keys it is an unbounded memory leak any client can
drive by rotating source addresses.

Limits are per-instance because the state is in memory. The limiter sits behind
a small interface so swapping in Redis for multi-replica deployments is a
one-file change.

## Configuration

Every setting is an environment variable, all validated at boot with **every**
problem reported at once. See [`.env.example`](.env.example) for the full list
with defaults.

The essentials:

| Variable | Default | Notes |
|---|---|---|
| `CALC_ENV` | `development` | `production` enables the hardened refusals above |
| `CALC_PORT` | `8080` | |
| `CALC_JWT_SECRET` | *generated* | Required in production, ≥32 bytes. In development a random per-process secret is minted and every restart invalidates outstanding tokens |
| `CALC_CLIENTS` | — | `id:bcryptHash,...` — mint hashes with `make hash SECRET=...` |
| `CALC_CLIENTS_PLAINTEXT` | — | `id:secret,...`, development only |
| `CALC_CORS_ORIGINS` | `http://localhost:5173` | |
| `CALC_RATE_LIMIT_RPS` / `_BURST` | `20` / `40` | Per authenticated client |
| `CALC_AUTH_RATE_LIMIT_RPS` / `_BURST` | `1` / `5` | Per source address |
| `CALC_MAX_EXPRESSION_LENGTH` | `256` | |
| `CALC_MAX_DEPTH` | `32` | Bounds parser recursion |

The depth cap is not cosmetic: a recursive-descent parser without one is a
stack-overflow denial of service, and a stack overflow in Go takes down the
whole process rather than one request.

## Testing

```bash
make test          # race detector across every package
make cover         # gated at 85%; currently 89.7%
make cover-html    # writes coverage.html and opens it
make cover-report  # per-function coverage in the terminal
make fuzz          # FUZZTIME=2m make fuzz for a longer run
```

`make cover-html` writes `backend/coverage.html`. From the repository root,
`make coverage-html` builds that report and the frontend's together and prints
both `file://` paths. Neither is committed — they are generated artefacts that
go stale immediately, so a link to one in the repository would be a dead link.

Table-driven throughout. `FuzzParse` asserts the two properties a hand-written
parser most often breaks: it never panics, and it never returns a success the
transport layer cannot encode.

Coverage is measured over `./internal/...`. `cmd/` is excluded deliberately —
`main` is wiring, and the way to keep it honest is to keep it thin, not to write
a test asserting that the wiring is wired.

## The images

Run instructions are [at the top](#running-it--docker-is-the-preferred-way);
this section is what the files actually do.

**[`Dockerfile`](Dockerfile)** — three stages. `build` compiles a static binary
with `CGO_ENABLED=0` and `-trimpath`; `runtime` copies just that binary onto
`gcr.io/distroless/static`, which carries no shell and no package manager. The
`test` stage is built only when asked for:

```bash
docker build --target test .    # vet, full suite, coverage gate — in the image
```

**[`../docker-compose.yml`](../docker-compose.yml)** — at the repository root,
building `./backend` and `./frontend`. Both services declare healthchecks, and
the frontend waits on `condition: service_healthy` so it never comes up pointing
at a dead API.

`CALC_TRUSTED_PROXIES` is set to the compose bridge network, so the API reads
the real client address out of the `X-Forwarded-For` that nginx sets rather than
attributing every request to the proxy.

⚠️ **The compose file carries development credentials** —
`CALC_CLIENTS_PLAINTEXT=web:dev-secret` and no `CALC_JWT_SECRET`. That is
deliberate and safe locally, because the server refuses to start with either of
those settings when `CALC_ENV=production`. A real deployment supplies
`CALC_JWT_SECRET` and `CALC_CLIENTS` (bcrypt hashes, via `make hash`) from a
secret store.
