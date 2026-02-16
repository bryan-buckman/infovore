# Golang - Learning Syllabus

## Overview

Go (Golang) is Google's statically typed, compiled language designed for simplicity, concurrency, and reliability at scale. Coming from Python/JS, you'll discover a language that trades dynamic flexibility for blazing speed, explicit error handling, and built-in concurrency primitives. This syllabus takes you from Go fundamentals to confidently building production-ready services — covering theory, hands-on projects, real-world patterns, and interview readiness.

## Prerequisites

- Comfortable writing programs in Python or JavaScript (functions, loops, data structures)
- Basic understanding of how the web works (HTTP, APIs, JSON)
- Familiarity with a terminal/command line
- Git basics (clone, commit, push)
- A code editor (VS Code with Go extension recommended)

## Learning Objectives

By the end, you will:

1. Understand Go's type system, interfaces, and how they differ from Python/JS
2. Write idiomatic Go code following community conventions
3. Master goroutines, channels, and concurrent programming patterns
4. Build and deploy production-ready REST APIs and CLI tools
5. Write comprehensive tests (unit, integration, table-driven, benchmarks)
6. Understand Go's memory model, garbage collector, and performance characteristics
7. Apply common Go design patterns and project structure conventions
8. Confidently answer Go interview questions on concurrency, interfaces, and error handling

## Learning Path

### Phase 1: Go Foundations (Weeks 1-2)
*Goal: Think in Go — understand the fundamental differences from Python/JS*

- [ ] **1.1 Environment & Toolchain** - Installing Go, GOPATH vs Go modules, `go run`/`go build`/`go mod init`, workspace layout
- [ ] **1.2 Variables, Types & Constants** - Static typing, type inference (`:=`), zero values, basic types (`int`, `string`, `bool`, `float64`), type conversions (no implicit casting!)
- [ ] **1.3 Control Flow** - `if`/`else` (no parentheses!), `for` (Go's only loop), `switch` (no fallthrough by default), `defer`
- [ ] **1.4 Functions & Multiple Returns** - Function syntax, multiple return values, named returns, variadic functions, first-class functions
- [ ] **1.5 Data Structures** - Arrays vs slices (critical distinction!), maps, `make()` vs literals, slice internals (len vs cap)
- [ ] **1.6 Strings, Runes & UTF-8** - String immutability, byte slices vs rune slices, `strings` and `strconv` packages
- [ ] **1.7 Pointers** - Pointer basics (`&` and `*`), pass-by-value semantics, when to use pointers vs values, nil pointers
- [ ] **1.8 Structs & Methods** - Defining structs, method receivers (value vs pointer), embedding (composition over inheritance)
- [ ] **1.9 Packages & Visibility** - Package organization, exported vs unexported (capital letter rule), `import`, `init()` functions
- [ ] **1.10 Error Handling Philosophy** - `error` interface, `if err != nil` pattern, `errors.New()`, `fmt.Errorf()` with `%w`, why Go doesn't have exceptions
- [ ] 🔨 **Project: CLI Task Manager** - Build a command-line todo app that reads/writes tasks to a JSON file. Covers: structs, slices, file I/O, error handling, packages, JSON marshaling

### Phase 2: Intermediate Go (Weeks 3-4)
*Goal: Write real Go — interfaces, concurrency, and testing*

- [ ] **2.1 Interfaces** - Implicit interface satisfaction, empty interface (`any`/`interface{}`), type assertions, type switches, common interfaces (`io.Reader`, `io.Writer`, `fmt.Stringer`, `error`)
- [ ] **2.2 Generics (Go 1.18+)** - Type parameters, constraints, `comparable`, when to use generics vs interfaces
- [ ] **2.3 Goroutines** - Launching goroutines, how they differ from threads/async-await, goroutine lifecycle, the Go scheduler (GMP model basics)
- [ ] **2.4 Channels** - Unbuffered vs buffered channels, directional channels, `range` over channels, `select` statement, channel patterns (fan-in, fan-out, pipeline)
- [ ] **2.5 Sync Primitives** - `sync.Mutex`, `sync.RWMutex`, `sync.WaitGroup`, `sync.Once`, `sync.Map` — when channels vs mutexes
- [ ] **2.6 Context Package** - `context.Background()`, `context.WithCancel`, `context.WithTimeout`, `context.WithValue`, propagating context through call chains
- [ ] **2.7 Testing Fundamentals** - `testing` package, table-driven tests, `t.Run()` subtests, `t.Helper()`, test naming conventions, `go test ./...`
- [ ] **2.8 Advanced Testing** - Benchmarks (`b.N`), fuzzing, test fixtures, mocking with interfaces, `httptest` package, test coverage (`-cover`)
- [ ] **2.9 Error Wrapping & Sentinel Errors** - `errors.Is()`, `errors.As()`, `errors.Unwrap()`, defining sentinel errors, custom error types, error wrapping chains
- [ ] **2.10 Standard Library Deep Dive** - `io`, `os`, `net/http`, `encoding/json`, `time`, `log/slog`, `regexp`, `sort` — the stdlib is your framework
- [ ] 🔨 **Project: Concurrent Web Scraper** - Build a web scraper that fetches multiple URLs concurrently using goroutines and channels. Includes: rate limiting, context cancellation, graceful shutdown, results aggregation, and full test suite

### Phase 3: Production Go (Weeks 5-7)
*Goal: Build real-world services — APIs, databases, and deployment*

- [ ] **3.1 Project Structure** - Flat vs layered structure, internal packages, `cmd/` pattern, dependency injection without frameworks
- [ ] **3.2 HTTP Servers & Routing** - `net/http` server, `http.Handler` and `http.HandlerFunc`, middleware pattern, Go 1.22+ enhanced routing, popular routers (chi, gorilla/mux)
- [ ] **3.3 JSON & API Design** - Struct tags, custom marshalers, request validation, API versioning, OpenAPI/Swagger
- [ ] **3.4 Database Access** - `database/sql` package, connection pooling, prepared statements, `sqlx` for convenience, migrations, repository pattern
- [ ] **3.5 Configuration & Environment** - Environment variables, config structs, Viper/envconfig, 12-factor app principles
- [ ] **3.6 Logging & Observability** - `log/slog` (structured logging), log levels, request tracing, basic metrics
- [ ] **3.7 Authentication & Middleware** - JWT handling, middleware chains, CORS, rate limiting, graceful shutdown with signals
- [ ] **3.8 Go Modules & Dependencies** - `go.mod`/`go.sum`, semantic versioning, `go mod tidy`, vendoring, private modules, major version suffixes
- [ ] **3.9 Docker & Deployment** - Multi-stage Docker builds, minimal container images, health checks, graceful shutdown, CI/CD basics
- [ ] 🔨 **Project: REST API Bookshelf Service** - Build a complete CRUD REST API for a book collection with: PostgreSQL storage, JWT auth, structured logging, middleware, Docker deployment, integration tests, and Makefile automation

### Phase 4: Mastery & Interview Readiness (Weeks 8-9)
*Goal: Deep understanding and confident articulation*

- [ ] **4.1 Memory & Performance** - Stack vs heap allocation, escape analysis (`go build -gcflags="-m"`), garbage collector tuning, `pprof` profiling, benchmarking patterns
- [ ] **4.2 Concurrency Patterns** - Worker pools, pipeline pattern, fan-out/fan-in, semaphore pattern, errgroup, rate limiting, circuit breaker
- [ ] **4.3 Reflection & Code Generation** - `reflect` package basics, struct tags at runtime, `go generate`, code generation tools
- [ ] **4.4 Go Internals** - How interfaces work internally (itab), slice header structure, string internals, the Go scheduler in depth, channel implementation
- [ ] **4.5 Common Pitfalls & Gotchas** - Loop variable capture, nil interface vs nil pointer, slice append gotchas, goroutine leaks, data races, `defer` in loops
- [ ] **4.6 Interview Problem Patterns** - Concurrency problems (dining philosophers, producer-consumer), system design in Go, API design questions, debugging race conditions
- [ ] **4.7 Design Patterns in Go** - Functional options, builder pattern, strategy via interfaces, observer with channels, singleton with `sync.Once`
- [ ] 🔨 **Project: Interview Portfolio Piece** - Build a URL shortener service with: Redis caching, rate limiting, analytics pipeline using goroutines, comprehensive tests, and clean architecture. This becomes your "show me your Go code" answer

## Teaching Milestones

- **After Phase 1:** Explain to someone: "Why does Go use `if err != nil` instead of try/catch? What's the philosophy behind it?" and "How are slices different from arrays, and why does it matter?"
- **After Phase 2:** Teach a peer: "When would you use a channel vs a mutex? Walk me through a real example." and "How do Go interfaces work differently from TypeScript/Java interfaces?"
- **After Phase 3:** Explain to a team: "How would you structure a production Go API? Walk through the architecture decisions." and "How does Go's `context` package help manage request lifecycles?"
- **After Phase 4:** Whiteboard session: "Design a concurrent pipeline that processes data from multiple sources with rate limiting and graceful shutdown."

## Resources

### Official
- [A Tour of Go](https://go.dev/tour/) — Interactive introduction (start here)
- [Effective Go](https://go.dev/doc/effective_go) — Official style and idiom guide
- [Go by Example](https://gobyexample.com/) — Annotated example programs
- [Go Standard Library Docs](https://pkg.go.dev/std) — Reference for all stdlib packages

### Books
- *The Go Programming Language* by Donovan & Kernighan — The definitive reference
- *Learning Go* by Jon Bodner (O'Reilly) — Great for developers coming from other languages
- *Concurrency in Go* by Katherine Cox-Buday — Deep dive into goroutines and channels

### Practice & Community
- [Exercism Go Track](https://exercism.org/tracks/go) — Mentored coding exercises
- [Go Playground](https://go.dev/play/) — Quick experimentation in browser
- [Gophercises](https://gophercises.com/) — Project-based Go exercises
- [r/golang](https://reddit.com/r/golang) — Community discussion

### Coming from Python/JS
- [Go for Python Programmers](https://golang-for-python-programmers.readthedocs.io/) — Side-by-side comparisons
- Key mental shifts: static types, explicit error handling, composition over inheritance, no classes, goroutines != async/await

## Success Criteria

- [ ] Can explain Go's type system and interfaces without reference
- [ ] Can write concurrent code with goroutines and channels confidently
- [ ] Can build and deploy a production REST API from scratch
- [ ] Completed all 4 🔨 hands-on projects
- [ ] Taught key concepts to someone else (or rubber duck)
- [ ] Can articulate Go vs Python/JS trade-offs in an interview setting
- [ ] All Phase 1-3 concepts reviewed via spaced repetition at least once
- [ ] Can identify and fix common Go pitfalls (race conditions, goroutine leaks, nil interfaces)
