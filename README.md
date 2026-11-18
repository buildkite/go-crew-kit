# go-crew-kit

Shared Go packages for Buildkite. Requires Go 1.23 or newer.

## filewrite

Use [`filewrite`](filewrite/filewrite.go) for bytes-based file creation and replacement on Unix filesystems with directory syncing support:

- `CreateExclusive(path, data, mode)` refuses existing destinations, applies exactly `mode.Perm()`, writes, syncs and closes the new file. The destination is visible while writing. On failure it attempts removal and preserves cleanup errors, but partial files can remain after a crash or failed cleanup. It does not sync the parent directory.
- `Replace(ctx, path, data, tempPattern)` writes and syncs a private `0600` staging file beside the destination, renames it into place and syncs the parent. Always inspect its `published` result even on error: a failed directory sync does not undo publication. Cancellation is checked before staging and immediately before rename, not after publication.
- `SyncDir(path)` opens, syncs and closes a directory, preserving sync and close errors. Use it after successful exclusive creation when you need to confirm the new directory entry, or within your own staged-directory publication sequence.

Parents must already exist. You own directory permissions, conflicting-write serialization, encoding and multi-file recovery. The package does not interrupt blocked filesystem calls, provide multi-file transactions or promise stronger durability than filesystem rename and sync semantics. Errors preserve causes through `errors.Is` and `errors.As` and may contain paths. See the [example](filewrite/filewrite_example_test.go) for creation and replacement.

## hostname

Use [`hostname.Valid`](hostname/hostname.go) to check ASCII hostname label syntax without normalization or DNS lookup. You control IP-literal handling, case normalization, trailing dots, wildcards, and ports in your application. Numeric dotted names meet the label grammar, so this check alone doesn't distinguish DNS names from IP literals.

## publicip

Use [`publicip.IsPublic`](publicip/publicip.go) to apply a conservative public-address policy to a `netip.Addr`. It rejects invalid and zoned addresses and checks IPv4-mapped IPv6 as IPv4. See the package documentation and exclusion list for the policy's scope and limitations.

For outbound SSRF protection, you must also authorize destinations, validate every DNS answer, and dial only the vetted addresses without resolving the name again. Keep internal-network exceptions and application-specific connection policy in your application.

## outbound

Use [`outbound.Dialer`](outbound/dial.go) as an `http.Transport.DialContext` hook to authorize destinations, validate every DNS answer, and connect only to the vetted IPs. A missing authorization callback denies all destinations. Authorized destinations use `publicip.IsPublic` unless your callback supplies an address predicate, such as a configured CIDR's `Contains` method. Invalid and zoned addresses are always rejected.

One timeout covers DNS and all connection attempts. Each attempt gets a share of the remaining time so later addresses can be tried. See the [transport example](outbound/dial_example_test.go) for public and internal destinations.

Use [`outbound.Classify`](outbound/errors.go) for stable destination-denied, address-denied, DNS, connection, cancellation, timeout, TLS, and generic transport categories. Dial errors preserve their causes through `errors.Is` and `errors.As`. The dial stage is available through `*outbound.Error` even when the classifier reports cancellation or timeout. Typed certificate, TLS record, and TLS alert failures are recognized. Untyped TLS failures remain generic transport errors rather than being identified by message text.

You still own host-and-port canonicalization, destination rules, credential forwarding, redirects, TLS verification, connection reuse, and response status or retry decisions. Disable environment proxies when configuring the HTTP transport so they don't bypass destination enforcement. Keep the hostname in request URLs for TLS verification and don't log raw errors, which may contain sensitive details. The dialer checks each new connection, not each request on a reused connection.

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

Buildkite runs formatting checks, native and 32-bit (386) tests, race tests, and vet on Go 1.23 using [the pipeline configuration](.buildkite/pipeline.yml).

Run these checks from the repository root, including on Go 1.23:

```sh
go test ./...
go test -race ./...
go vet ./...
```
