## Context

See proposal.md - Why.

`config.Load` handles each variable its own way. `DATABASE_URL` gets an inline
emptiness check. `WORKERS`, `MAX_UPLOAD_BYTES` and `PDF_DPI` each get a
near-identical six-line parse block. The five dewarp variables get a map, a
counter, and a sort in `loadDewarp`.

`loadDewarp` reports the first shape of failure it finds. It accepts an empty
set, rejects a partial set, and accepts a full set. The empty case is what this
change removes. The partial case is the rule that stays.

`objectstore.New` signals "not configured" by returning nil. Three call sites
read that nil: `main` skips the worker, `router` documents the field, and
`actions` answers 503.

## Goals

- One error lists every misconfigured variable, whatever its type.
- No code path represents an absent object store.
- `config.go` shrinks. Each variable reads as one line that names its own rule.

## Non-Goals

- No change to how dewarp jobs run. Only startup and configuration change.
- No local S3 in `docker-compose.dev.yml`. Development uses real credentials.
- No configuration library, and no change to how `main` reports a failed `Load`.

## Decisions

### Collect every configuration error, then return one

`Load` builds an accumulator, reads every variable through it, and returns
`errors.Join` of what failed.

```go
type env struct{ errs []error }

func (e *env) require(key string) string {
	value := os.Getenv(key)
	if value == "" {
		e.errs = append(e.errs, fmt.Errorf("%s is not set", key))
	}
	return value
}

func (e *env) orElse(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (e *env) positiveOrElse(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 {
		e.errs = append(e.errs, fmt.Errorf("%s must be a positive integer, got %q", key, raw))
		return fallback
	}
	return value
}

func (e *env) err() error { return errors.Join(e.errs...) }
```

`Load` then reads as one struct literal, and returns `Config{}` with the joined
error when `err()` is not nil. It never returns the partly built value.

Alternatives:

- **`mustEnv` that panics.** Reports one variable per restart. An operator with
  five empty variables restarts five times. Panic also suits programmer error,
  not operator error, and it would break the `t.Setenv` tests.
- **Return on the first missing variable.** Same one-at-a-time cost, without the
  panic.
- **A struct-tag library such as `envconfig`.** A dependency for six variables.
  The trailing-slash trim, the positive-integer rule, and the region default
  stay hand-written either way.

`errors.Join` returns nil for an empty slice, so `err()` needs no guard. It
formats one cause per line, which reads well in a startup log.

Report order is declaration order, which follows `.env.example`. The `sort` call
and the `sort` import go.

### `positiveOrElse` works in int64, and callers cast

Go methods take no type parameters. A generic helper would have to be a free
function taking `*env`, which breaks the method chain in the struct literal. Two
`int(...)` casts at `Workers` and `PDFDPI` cost less than that.

### Empty means missing

`os.Getenv` cannot tell unset from empty, and `os.LookupEnv` does not help here.
Compose sets `${AWS_BUCKET}` to an empty string when the host holds no value.
The variable is present and empty, not absent. Treating empty as missing is the
correct rule, not a shortcut.

### `AWS_ENDPOINT_URL` stays optional, `AWS_REGION` keeps its default

An empty `AWS_ENDPOINT_URL` means real AWS. It is a value, not an omission, so
`require` does not apply to it. `AWS_REGION` has a working default and the same
argument. The required set stays the five that `loadDewarp` already names.

### Compose passes bare `${AWS_BUCKET}`, not `${AWS_BUCKET:?}`

`:?` makes compose stop at the first empty variable. That hides the other four
and undoes the accumulator. Bare `${AWS_BUCKET}` passes the empty value through,
and the server names all five at once.

`docker-compose.yml` marks `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
and `ALLOWED_ORIGINS` with `:?`, so the file holds two ways to say "required".
The postgres three need `:?` because compose reads them itself, in the
healthcheck and in `DATABASE_URL`. `ALLOWED_ORIGINS` only passes through, so it
is a true precedent against this choice. The one-error rule wins anyway: the
server reads the five dewarp variables and reports them better than compose
does.

### `objectstore.New` returns a store and no error

Every input it needs is already validated by `Load`. Adding an error return
would push a check back into `main` for a case that cannot happen. `complete()`
and the nil return both go, and `Store` stops being a nullable type.

### `fakerunpod` validates the job payload itself

`fakerunpod` calls `New` with credentials from the request body, not from the
environment, and reads the nil to reject a job that carries none. That is
validation of untrusted input, not a configuration check. Startup validation
cannot cover it, because the payload arrives long after start.

The tool therefore checks `input.AccessKeyID`, `input.SecretAccessKey` and
`input.Bucket` directly, and keeps the same error. This is the one place where
the nil return did real work.

Two control handlers, `controlObjects` and `controlPurge`, also read the nil.
They guard the store that `main` builds from the environment, not a payload, so
they answer 503 for a case that can no longer happen. Both checks go. The e2e
composes set every variable, so no test reaches that answer today.

### The removed requirement gets no replacement

"The server refuses to start without its configuration" is a deployment rule. It
is not behavior that `book-dewarp` promises to a user or a client. The README
and `.env.example` carry it instead.

## Risks / Trade-offs

- **A server running today without dewarp stops starting.** The operator sees a
  startup error naming the empty variables and must set them. Mitigation: the
  error lists all five at once, and the release note states the new requirement.
- **Development loses its zero-configuration start.** A copy of `.env.example`
  no longer boots the backend. Mitigation: README states that development needs
  real credentials. The e2e composes already set every variable and do not
  change.
- **An invalid numeric value still yields a fallback inside the accumulator.**
  A reader could mistake that value for the effective setting. Mitigation:
  `Load` discards the partial `Config` and returns `Config{}` with the error.
- **The five variables become five ways to fail a deploy.** Mitigation: this is
  the point of the change. A missing bucket now fails at start, not at the first
  dewarp job hours later.

## Migration Plan

1. Set the five variables in every deployment before rolling out the new image.
   The current image ignores them when they are complete, so this step is safe
   on its own and can land first.
2. Roll out the new image. A server with a complete set starts as before.
3. Rollback: redeploy the previous image. It accepts both the empty set and the
   complete set, so no configuration has to be undone.

No database migration. No API change.
