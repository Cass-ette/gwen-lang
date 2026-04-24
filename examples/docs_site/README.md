# Gwen Docs Site Prototype

Minimal repo-driven teaching site served by Gwen itself.

## Run

```bash
go run ./cmd/gwen run examples/docs_site/main.gw
```

Optional custom address:

```bash
go run ./cmd/gwen run examples/docs_site/main.gw 127.0.0.1:8091
```

Then open:

- `http://127.0.0.1:8090/`
- `http://127.0.0.1:8090/api/health`

## What This Prototype Proves

- Gwen can serve a read-heavy teaching site backend today
- The site can discover docs and examples directly from the repository tree
- Markdown pages and Gwen source can be exposed through Gwen JSON APIs without maintaining a second content mirror
- Search can run over the real repo corpus instead of hand-written summaries
