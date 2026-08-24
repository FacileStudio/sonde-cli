# Development

Requires Go 1.26 (via [mise](https://mise.jdx.dev)).

```sh
mise run build   # go build -o sonde .
mise run check   # gofmt, vet, test, build
```

Module path: `github.com/FacileStudio/sonde-cli`. Layout follows the suite's other Go CLIs (`mycelium`, `antenne-cli`): cobra commands in `cmd/`, client logic in `internal/api`, config in `internal/config`, output glyphs in `internal/ui`.
