# Calculator

React 19 + TypeScript front end for the calculation API in
[`../backend`](../backend). It composes expressions; the Go service evaluates
them.

One keypad: the four operations, brackets, percent, square root and
exponentiation. Nothing more — the brief asks for correctness, clarity and
maintainability over extra features, and every additional operation is surface
area a reviewer has to read.

```bash
npm install
npm run dev      # http://localhost:5173, proxying /v1 to the API
npm test         # unit tests
npm run build    # typecheck + production bundle
```

The API must be running:

```bash
cd ../backend && make run
```

## There is no arithmetic in this repo

The engine moved to Go. `src/core/arithmetic.ts`, `functions.ts`, `constants.ts`
and the evaluating reducer are gone — about 850 lines. What remains is text
editing: keys append to an expression, `=` posts it, the server answers.

That is the point. Two engines meant two sets of rounding rules, two domain
tables and two chances to disagree with each other. Now there is one.

## Architecture

```
src/
├── api/
│   ├── client.ts              # fetch wrapper, token lifecycle, typed errors
│   ├── config.ts              # the configured singleton
│   └── types.ts               # the wire contract
├── core/
│   └── expression.ts          # pure reducer over text — no arithmetic
├── hooks/
│   ├── useCalculator.ts       # binds the reducer to React and to the API
│   └── useKeyboardInput.ts    # physical keyboard → the same actions
├── components/                # view + styles + barrel per component
│   ├── Calculator/  Display/  Keypad/  Key/
├── styles/                    # design tokens + global reset
└── types/                     # domain + keypad types
```

**Data flow.** `Calculator` owns the state. Pointer presses and keystrokes both
dispatch the same actions; `useCalculator` intercepts `evaluate` and turns it
into a request. Every other action is synchronous and local, so typing never
waits on the network.

**Keypad config.** Labels, ARIA names and keyboard bindings are data, in
[`Keypad/keys.ts`](src/components/Keypad/keys.ts). Each key carries the text it
inserts, so adding one is a single object.

## What changed when the backend arrived

| | Before | Now |
|---|---|---|
| `2 + 3 × 4` | 20 — strictly left to right | **14** — real precedence |
| Brackets | none | explicit `(` and `)`, with `)` disabled while nothing is open |
| `√` and `^` | absent | on the keypad's function row |
| `%` | `200 + 10%` was 220 | **postfix ÷100**, so `50%` is `0.5` |
| `+/−` | a key | gone — `−` at the start of a term already negates |
| Errors | inline message | server message, **with the bad character highlighted** |

The percent change is the one most likely to surprise a returning user: the old
relative percent was a UI convention, and the grammar now takes `%` literally.

## Talking to the API

`src/api/client.ts` owns the token lifecycle: mint on first use, cache until 30
seconds before expiry, share one in-flight request between concurrent callers,
and retry once on a 401 — the development server mints a new signing secret on
every restart, so a stale token is routine rather than exceptional.

Failures arrive typed. `CalculatorApiError` carries the server's `code`,
`message` and `position`; `CalculatorNetworkError` covers unreachable and
timed-out. The display shows the message and marks the character at `position`,
leaving the expression on screen so it can be fixed rather than retyped.

⚠️ **The client secret ships in the bundle and is public.** Any user can read
it. That is inherent to a browser SPA: the token gives the server per-client
attribution and keeps non-browser consumers out, while the real defence for this
surface is the API's rate limiting, CORS allowlist and payload caps. Never put a
production secret in a `VITE_` variable.

Nothing is cross-origin — Vite proxies `/v1` in development and nginx does it in
the container — so CORS never enters the picture. See
[`.env.example`](.env.example).

## Testing

```bash
npm test            # 97 tests
npm run test:watch  # watch mode
npm run test:coverage
npm run build       # typecheck + bundle
```

Coverage is gated at **85%** across statements, branches, functions and lines —
the same threshold as the Go service, so neither side can quietly fall behind.
It currently sits at 97.7% statements, 95.8% branches, 97.7% functions and 99.4%
lines. The gate is configured in [`vite.config.ts`](vite.config.ts); barrels,
`main.tsx`, type-only modules and `api/config.ts` are excluded as wiring with no
behaviour to exercise.

`npm run test:coverage` writes a browsable report to
[`coverage/index.html`](coverage/index.html), which is committed. From the
repository root, `make coverage-html` regenerates it alongside the backend's.

| Suite | What it pins down |
|---|---|
| [`core/expression.test.ts`](src/core/expression.test.ts) | The reducer: implicit products, token-aware backspace, what continues from a result versus what replaces it, and that every key's inserted text is removable in one backspace |
| [`api/client.test.ts`](src/api/client.test.ts) | Token cache, concurrent de-duplication, the single 401 retry, and every failure shape |
| [`hooks/useCalculator.test.ts`](src/hooks/useCalculator.test.ts) | The async path: bracket auto-closing before submit, error mapping, and the race guard |
| [`hooks/useKeyboardInput.test.ts`](src/hooks/useKeyboardInput.test.ts) | Bindings for both key faces, modifier keys ignored, listener cleanup |
| Component suites | The `)` key disabled at depth 0, `=` disabled mid-request, and the error mark landing on the right character |

The race guard is the one worth knowing about: a slow first request landing
after a fast second one would silently overwrite the newer result, and no amount
of clicking by hand would ever reveal it.

## Accessibility

Every key is a real `<button>` with an `aria-label`. The display is an `<output aria-live="polite">`
and failures are announced through `role="alert"`. Light and dark themes follow
the OS setting; motion is dropped under `prefers-reduced-motion`.

Keyboard: digits, `+` `-` `*` `/` `^` `q` (√) `(` `)` `%` `.`, `Enter`/`=` to
evaluate, `Backspace`, `Esc`/`c` to clear.

## Docker

Built and served by nginx, which proxies `/v1` to the API. Compose lives at the
repository root and is the preferred way to run the whole product:

```bash
cd .. && docker compose up --build     # frontend :8080, API :8081
```
