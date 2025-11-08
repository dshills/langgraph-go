# LangGraph-Go API Documentation

This directory contains generated HTML documentation for all LangGraph-Go packages using the Go `godoc` tool.

## Viewing Documentation

### Option 1: Open HTML Files Directly

Open any HTML file in your web browser:

```bash
# macOS
open docs/godoc/index.html

# Linux
xdg-open docs/godoc/index.html

# Windows
start docs/godoc/index.html
```

### Option 2: Serve with a Local Web Server

For better navigation and styling, serve the documentation locally:

```bash
# Using Python
cd docs/godoc
python3 -m http.server 8080

# Using Go
cd docs/godoc
go run -m http.server 8080

# Then open http://localhost:8080 in your browser
```

### Option 3: Use godoc Directly

Generate and serve documentation on-the-fly:

```bash
# Install godoc (if not already installed)
go install golang.org/x/tools/cmd/godoc@latest

# Serve documentation
godoc -http=:6060

# Open http://localhost:6060/pkg/github.com/dshills/langgraph-go/
```

## Package Documentation

| Package | File | Description |
|---------|------|-------------|
| **Main** | [index.html](./index.html) | Project overview and package index |
| **graph** | [graph.html](./graph.html) | Core workflow engine, nodes, and execution |
| **graph/emit** | [emit.html](./emit.html) | Event emission and observability (logging, OpenTelemetry) |
| **graph/store** | [store.html](./store.html) | Persistence layer (memory, SQLite, MySQL) |
| **graph/model** | [model.html](./model.html) | LLM integrations (OpenAI, Anthropic, Google) |
| **graph/tool** | [tool.html](./tool.html) | Tool abstractions and implementations |

## Documentation Sections

Each package documentation includes:

- **Package Overview**: High-level description and purpose
- **Index**: Quick reference of all exported types and functions
- **Constants & Variables**: Package-level declarations
- **Functions**: Top-level functions with signatures and descriptions
- **Types**: Structs, interfaces, and methods with detailed documentation
- **Examples**: Code examples (where available)

## Regenerating Documentation

To regenerate the HTML documentation:

```bash
# From project root
make godoc

# Or manually:
godoc -http=:6061 &
GODOC_PID=$!
mkdir -p docs/godoc
curl -s http://localhost:6061/pkg/github.com/dshills/langgraph-go/ -o docs/godoc/index.html
curl -s http://localhost:6061/pkg/github.com/dshills/langgraph-go/graph/ -o docs/godoc/graph.html
curl -s http://localhost:6061/pkg/github.com/dshills/langgraph-go/graph/emit/ -o docs/godoc/emit.html
curl -s http://localhost:6061/pkg/github.com/dshills/langgraph-go/graph/store/ -o docs/godoc/store.html
curl -s http://localhost:6061/pkg/github.com/dshills/langgraph-go/graph/model/ -o docs/godoc/model.html
curl -s http://localhost:6061/pkg/github.com/dshills/langgraph-go/graph/tool/ -o docs/godoc/tool.html
kill $GODOC_PID
```

## Online Documentation

The documentation is also available online at:

- **pkg.go.dev**: https://pkg.go.dev/github.com/dshills/langgraph-go
- **GitHub**: See source code at https://github.com/dshills/langgraph-go

## Last Generated

Generated: 2025-11-08
Go Version: 1.21+
Tool: golang.org/x/tools/cmd/godoc
