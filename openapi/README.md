# OpenAPI input

`openapi.json` is a vendored copy of the Nexrender API specification used to generate `internal/api/api.gen.go`.

Update it from the Nexrender API repository, review the specification diff, then run:

```bash
go generate ./internal/api
go test ./...
```

The command tree is not generated from this document. Multi-request uploads, polling, diagnosis, validation, and confirmation remain hand-written CLI workflows.
