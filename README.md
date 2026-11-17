# go-crew-kit

Shared Go packages for Buildkite. Requires Go 1.23 or newer.

## envconfig

Use [`envconfig`](envconfig/loader.go) to load an explicitly selected dotenv file and process environment overrides into an explicit table of field pointers. Set defaults before loading and perform application-specific validation afterward.

```go
listen := "127.0.0.1:8080"
timeout := 30 * time.Second

loader := envconfig.Loader{Prefix: "APP_"}
err := loader.Load(envFile, []envconfig.Setting{
    {Name: "LISTEN", Target: &listen},
    {Name: "TIMEOUT", Target: &timeout},
})
if err != nil {
    return err
}
```

Import `github.com/buildkite/go-crew-kit/envconfig` and `time` for this example. Pass an empty `envFile` to skip file loading. See the [executable example](envconfig/example_test.go) for a complete call with a custom environment lookup.

### Loading contract

- Process variables override file values, including explicitly empty values. Missing settings retain their existing target values. Unknown process variables are ignored.
- File syntax and names are checked before any values are applied. Unknown and duplicate file keys are rejected even if the environment overrides them. Only each setting's winning value is parsed.
- Files are never discovered automatically, and loading does not change the process environment. Relative file paths use the working directory.
- Targets can be pointers to `string`, `bool`, `time.Duration`, `int`, `int64`, or `[]string`. Booleans accept only `true` and `false`. Integers accept unsigned decimal spelling without leading zeros except `0`, bounded by the target's signed integer range. Durations use `time.ParseDuration`. Lists are comma-separated, with whitespace trimmed from each entry. An empty value clears a list, but empty entries within a list are rejected.
- Loading isn't transactional. Discard your partially loaded configuration after an error. Setting names must be unique and targets must be non-nil pointers of supported types. An unsupported target type is a programming error, not a configuration error.

### File syntax

Files accept blank lines, full-line `#` comments, and `KEY=value` assignments. Leading and trailing whitespace outside quotes is ignored. Single and double quotes enclose literal bytes without interpolation or escape processing. An unquoted `#` starts a comment only at the beginning of a value or after a space or tab. After a quoted value, only whitespace and an optional comment are accepted.

`export`, multiline values, and NUL bytes aren't supported. Lines must fit within the 64 KiB scanner buffer, including the line ending. Files can use LF or CRLF endings.

### Errors

Use `errors.Is(err, envconfig.ErrInvalid)` to identify syntax, setting-name, and value errors. Diagnostics identify a setting or file line without echoing configuration values or unknown file keys. File I/O errors preserve their underlying cause and may include the selected path.

## Development

Run these checks from the repository root, including on Go 1.23:

```sh
go test ./...
go test -race ./...
go vet ./...
```
