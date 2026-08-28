# Repository notes

## Product contract

- The binary and command prefix are `nexrender`.
- Release and installer URLs use `https://github.com/nexrender/nexrender-cli`.
- Human output may change for readability. JSON fields, error codes, and exit codes are compatibility-sensitive.
- OpenAPI generates transport types and methods. Hand-design commands around operator workflows.
- Never send the Nexrender bearer token to presigned upload or download hosts.
- Do not retry create, update, cancel, delete, or upload requests after ambiguous failures.
- Do not print API tokens or secret values.

## Validation

Run before handing off changes:

```bash
go generate ./internal/api
gofmt -w cmd internal openapi skills
go test ./...
go vet ./...
go build ./cmd/nexrender
bash -n scripts/install.sh
```

Template-upload tests must keep asserting that Nexrender authorization is absent from the presigned request.

## Generated files

`internal/api/api.gen.go` is generated from `openapi/openapi.json` with the pinned generator in `internal/api/generate.go`. Do not hand-edit it.
