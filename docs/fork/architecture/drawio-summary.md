# Draw.io delivery note

Derived on 2026-10-13. Produced with `architecture-visualization:drawio`
(delivery layer) on top of the evidence model in `extraction.model.json`.

## What is canonical, and what is editable

| File | Role |
|---|---|
| `extraction.model.json` | **Canonical architecture model** — nodes, edges, state, confidence, `sourceRefs` |
| `README.md` | View explanation, evidence table, assumptions, limitations |
| `extraction-boundary.dot`, `iam-seam-blockers.dot` | Generated Graphviz sources (reviewable text; DOT is the better format for a dense dependency graph) |
| `cmd-internal-extraction.drawio` | **Derived, editable delivery** — two pages, same ids as the model |
| `generate.py` | Regenerates all three renderings from the model (stdlib only) |

Open `cmd-internal-extraction.drawio` in Draw.io (this machine has
`/snap/bin/drawio`) to move boxes, recolor, or add annotations for a review
meeting. Editing the model means editing **JSON**, not the `.drawio`: node and
edge ids (`cmd`, `iamInCmd`, `e-cmd-iamInCmd`, …) are stable across both
renderings, so manual edits in Draw.io that change a fact will not survive the
next `generate.py` run and must be written back to `extraction.model.json`
first.

## Generation

```sh
cd docs/fork/architecture
python3 generate.py          # rewrites both .dot files and the .drawio
```

Model metadata is not dropped on the floor: every node carries
`state`/`confidence`/`sourceRefs` and every edge `type`/`sync`/`protocol`/`risk`
(where set)/`confidence`/`sourceRefs` into both renderings - as `tooltip`
attributes plus an adjacent comment, because a bare `mxCell` has no verified slot
for arbitrary fields, and the comment is the durable copy.

Validated: the `.drawio` parses as XML, 2 diagrams, 22 vertices, 18 edges;
re-running `generate.py` leaves the tree clean (the committed output is stable).
Graphviz itself is **not installed** in this environment (`dot: command not
found`), so the `.dot` files are generated but were not rendered or layout-
checked here — they are unverified as pictures, though the syntax is
mechanically emitted.

## MCP availability

No Draw.io MCP server is connected in this session (`mcp_list` for "drawio"
returns nothing), so nothing was opened or edited through MCP; the file was
written directly. To let an agent open diagrams in Draw.io instead:

```toml
[mcp_servers.drawio]
command = "npx"
args = ["-y", "@drawio/mcp"]
```

That package offers `open_drawio_mermaid` / `open_drawio_xml` /
`open_drawio_csv`. A live-editor variant (`drawio-mcp-server --editor`) can
inspect and change an already-open document, which is the wrong tool here: these
views are regenerated from a model, and interactive edits to generated output
would diverge from it. For private architecture content prefer a local or
self-hosted endpoint over the hosted one.

## What this artifact deliberately does not claim

- It is not a runtime view. Nothing here comes from telemetry, traces or a live
  cluster; every edge is a static import, a call site or a compile-probe result.
- The `target`/`proposed` nodes (`internal/siterep`, the injected `Deps` hooks,
  the 5-method object seam) are design intent recorded in `../backlog.md`, not
  code that exists.
- The blockers are stated as facts because they were measured; the *fixes*
  beside them are proposals and are labelled as such (`state=proposed`).
