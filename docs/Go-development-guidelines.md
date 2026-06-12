# Go Development Guidelines
## Project Stack
The following libraries were specified for reference in this project:
**User-Specified Libraries**:
- **Web Framework**: chi (v5.3.0) - Lightweight, idiomatic, composable HTTP router for building Go services - https://github.com/go-chi/chi
- **Database Driver**: pgx (v5.10.0) - Pure Go PostgreSQL driver and toolkit - https://github.com/jackc/pgx
- **Testing**: testify (v1.11.1) - Toolkit with assertions, mocking, and test suite support - https://github.com/stretchr/testify
**Auto-Populated Essential Tools**:
- **Formatting**: gofmt, goimports - Standard Go code formatter and import organizer
- **Linting**: go vet, staticcheck, golangci-lint - Official and community static analysis tools
- **Logging**: log/slog (Go 1.21+) - Structured logging package in Go standard library
- **Build Tool**: go build, make - Native Go build system and build automation
> **Note**: This section lists libraries for quick reference.
> All code examples in this guideline use standard library or language-native features.
> Principles and patterns apply regardless of library choices.
## 1. Core Principles

### 1.1 Philosophy and Style

Go prioritizes clarity, simplicity, and maintainability. Code is formatted automatically with `gofmt` to eliminate style debates. The compiler enforces correctness through static typing and explicit error handling.

```go
// Good: Clear, idiomatic Go
func greet(name string) string {
    return "Hello, " + name
}
```

- Use `gofmt` for automatic formatting (tab indentation, no line length limit)
- Follow idiomatic conventions from Effective Go and Google Go Style Guide
- Favor simplicity: clear code over clever abstractions
- Run `go vet` and `staticcheck` before committing

### 1.2 Clarity over Brevity

Names must communicate intent. Self-explanatory code reduces comment overhead.

```go
// Bad: Unclear naming
func f(x int) int {
    return x * 7
}

// Good: Intent-revealing names
func calculateWeeklySalary(dailyRate int) int {
    return dailyRate * 7
}
```

- Avoid premature optimization: write clear code first, profile later
- Use meaningful identifiers: `users`, `getUserByID`, `maxRetries`
- Prefer explicit error handling over panic/recover for routine errors

## 2. Project Initialization

### 2.1 Creating New Project

Initialize a Go module with a module path based on the repository URL:

```bash
mkdir my-service && cd my-service
go mod init github.com/username/my-service
```

Pin Go version and create main entry point:

```bash
go mod edit -go=1.23
touch main.go
```

### 2.2 Dependency Management

Go uses the module system with `go.mod` and `go.sum` for dependency management:

```bash
# Add a dependency
go get github.com/go-chi/chi/v5@latest

# Update all dependencies
go get -u ./...

# Remove unused dependencies
go mod tidy

# Verify dependencies
go mod verify

# Vendor dependencies
go mod vendor
```

## 3. Project Structure

### 3.1 Standard Directory Layout

Official Go guidance recommends a minimal, pragmatic structure. The `cmd/` and `internal/` patterns are recognized by the Go toolchain.

```
my-service/
  go.mod
  go.sum
  main.go
  cmd/
    server/
      main.go
  internal/
    handler/
      handler.go
    service/
      service.go
    repository/
      repository.go
  .env
  Makefile
  Dockerfile
  .gitignore
```

### 3.2 Package Organization

- One package per directory, named with lowercase only
- Use `internal/` to prevent external packages from importing internal code
- Place binary entry points in `cmd/<name>/main.go`
- Avoid generic package names like `util`, `common`, `helper`

### 3.3 Key Files

| File | Purpose |
|------|---------|
| `go.mod` | Module definition and dependency declarations |
| `go.sum` | Dependency checksums for verification |
| `main.go` | Application entry point (root or in cmd/) |
| `Makefile` | Build automation and common commands |

## 4. Container Development (Docker)

### 4.1 Container Philosophy

Docker ensures consistent environments across development, CI, and production. Use Docker for local development to avoid installing runtime dependencies directly.

### 4.2 Dockerfile for Development

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/server ./cmd/server

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
```

### 4.3 Docker Compose

```yaml
version: "3.9"
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://postgres:postgres@db:5432/mydb
    depends_on:
      db:
        condition: service_healthy
    volumes:
      - .:/app

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: mydb
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5
```

### 4.4 .dockerignore

```
.git
vendor/
*.md
*.log
tmp/
```

### 4.5 Essential Commands

| Command | Description |
|---------|-------------|
| `docker compose up -d` | Start environment |
| `docker compose logs -f` | View logs |
| `docker compose exec app go run ./cmd/server` | Run application |
| `docker compose exec app go test ./...` | Run tests |
| `docker compose exec app sh` | Interactive shell |
| `docker compose down` | Stop environment |

### 4.6 Makefile

```makefile
.PHONY: run test lint build

run:
	go run ./cmd/server

test:
	go test -v -race -count=1 ./...

lint:
	golangci-lint run ./...

build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

docker-up:
	docker compose up -d

docker-down:
	docker compose down
```

### 4.7 Best Practices

- Use multi-stage builds to keep images small
- Pin base image versions (e.g., `golang:1.23-alpine`)
- Use `CGO_ENABLED=0` for static binaries in Alpine
- Never run containers as root; use `USER 1000`
- Use `.dockerignore` to exclude unnecessary files

## 5. Naming Conventions

### 5.1 General Rules

Go uses `MixedCaps` (no underscores or hyphens in identifiers). Acronyms are all uppercase: `HTTP`, `URL`, `ID`.

| Element | Convention | Example |
|---------|-----------|---------|
| Package | Lowercase, single word | `http`, `handler`, `db` |
| Types | PascalCase | `User`, `HTTPClient` |
| Functions | PascalCase (exported), camelCase (unexported) | `GetUser`, `getUser` |
| Variables | camelCase | `userName`, `maxRetries` |
| Constants | PascalCase | `MaxTimeout`, `DefaultPort` |
| Files | snake_case | `user_handler.go` |
| Interfaces | -er suffix or `I` prefix | `Reader`, `Handler` |

### 5.2 Package Naming

```go
// Good: Short, descriptive package name
package user

// Bad: Generic, meaningless names
package util
package common
```

### 5.3 Acronyms and Initialisms

Consistently use the same case for acronyms:

```go
var userID int      // Not userId
func ParseURL()     // Not ParseUrl
type HTTPClient     // Not HttpClient
```

## 6. Types and Type System

### 6.1 Type Declaration

Go is statically typed with type inference via `:=`.

```go
type User struct {
    ID        int64
    Name      string
    Email     string
    CreatedAt time.Time
}

type UserID int64
type JSONMap map[string]any
type HandlerFunc func(http.ResponseWriter, *http.Request)

type Status string

const (
    StatusActive   Status = "active"
    StatusInactive Status = "inactive"
)
```

### 6.2 Type Safety

```go
// Good: Strong typing prevents misuse
type Meter float64
type Feet float64

func NewMeter(v float64) Meter { return Meter(v) }
func (m Meter) ToFeet() Feet { return Feet(m * 3.28084) }

// Bad: Primitive obsession leads to errors in call sites
func process(distance float64) {}
```

### 6.3 Allocation and Initialization

```go
// Using composite literals
u := User{ID: 1, Name: "Alice"}

// Using new (zero values)
ptr := new(User)

// Using make for slices, maps, channels
users := make([]User, 0, 100)
m := make(map[string]int)
ch := make(chan int, 10)
```

## 7. Functions and Methods

### 7.1 Signatures

```go
// Function declaration with parameters and return types
func CalculateTotal(prices []float64, taxRate float64) (float64, error) {
    if taxRate < 0 {
        return 0, fmt.Errorf("tax rate must be non-negative: %f", taxRate)
    }
    var sum float64
    for _, p := range prices {
        sum += p
    }
    return sum * (1 + taxRate), nil
}

// Method on a type
func (u *User) FullName() string {
    return u.Name
}
```

### 7.2 Returns and Errors

```go
// Good: Explicit error handling in signature
func Divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
}

// Bad: Returning magic values or ignoring errors
func Divide(a, b float64) float64 {
    if b == 0 {
        return -1 // magic value, caller must check
    }
    return a / b
}
```

### 7.3 Best Practices

- Single responsibility: one function, one job
- Limit parameters to 3-4; use a config struct for more
- Avoid hidden side effects (logging, writing to globals)
- Return early to reduce nesting

```go
// Good: Early return pattern
func ProcessOrder(o Order) error {
    if !o.IsValid() {
        return ErrInvalidOrder
    }
    // process order
    return nil
}
```

## 8. Error Handling

### 8.1 Philosophy

Go uses explicit error values (not exceptions). Errors are returned as the last return value. The caller must handle or propagate them.

```go
// Creating errors
var ErrNotFound = errors.New("resource not found")

// Wrapping errors with context
func GetUser(id int64) (*User, error) {
    user, err := db.QueryUser(id)
    if err != nil {
        return nil, fmt.Errorf("get user %d: %w", id, err)
    }
    return user, nil
}

// Custom error types
type ValidationError struct {
    Field string
    Value any
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("invalid field %s: %v", e.Field, e.Value)
}
```

### 8.2 Conventions

```go
// Good: Handle error explicitly, provide context
result, err := doSomething()
if err != nil {
    return fmt.Errorf("do something: %w", err)
}

// Bad: Silent error swallowing
result, _ := doSomething()
result, err := doSomething()
if err != nil {
    return // silently drops error
}
```

### 8.3 Best Practices

- Never ignore errors: check every error return
- Add context with `fmt.Errorf("operation: %w", err)`
- Use sentinel errors (`var ErrX = errors.New("...")`) for expected failures
- Use custom error types for domain-specific errors
- Log errors at I/O boundaries, not in every layer
- Use `errors.Is()` and `errors.As()` for error inspection

## 9. Concurrency and Parallelism

### 9.1 Concurrency Model

Go uses goroutines (lightweight threads) and channels for communication. Goroutines are multiplexed onto OS threads by the Go runtime.

```go
// Start a goroutine
go func() {
    fmt.Println("running concurrently")
}()

// Channel communication
ch := make(chan int)
go func() {
    ch <- 42
}()
value := <-ch
```

### 9.2 Synchronization

```go
// Mutex for shared state protection
type Counter struct {
    mu    sync.Mutex
    value int
}

func (c *Counter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.value++
}

// WaitGroup for waiting on multiple goroutines
var wg sync.WaitGroup
for i := 0; i < 5; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        fmt.Println("worker", id)
    }(i)
}
wg.Wait()
```

### 9.3 Best Practices

- Never start a goroutine without knowing when it stops
- Use `context.Context` for cancellation and deadlines
- Channel ownership: sender closes, receiver never closes
- Use `sync/atomic` for simple counters and flags

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

select {
case result := <-ch:
    fmt.Println(result)
case <-ctx.Done():
    fmt.Println("timeout")
}
```

### 9.4 Common Pitfalls

- Goroutine leaks: goroutines blocked forever on channel sends/receives
- Data races: concurrent writes without synchronization (use `go run -race`)
- Closing channels from the receiver side
- Using `time.Sleep` for synchronization instead of channels or WaitGroup

## 10. Interfaces and Abstractions

### 10.1 Interface Design

Go interfaces are satisfied implicitly (duck typing). Design small, focused interfaces with 1-3 methods.

```go
// Good: Small, focused interface
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

// Composed interface
type ReadWriter interface {
    Reader
    Writer
}
```

### 10.2 Implementation

Types satisfy interfaces implicitly. No explicit `implements` keyword.

```go
type FileStore struct {
    path string
}

func (f *FileStore) Read(p []byte) (int, error) {
    return os.Open(f.path).Read(p)
}

// Compile-time check (optional but recommended)
var _ Reader = (*FileStore)(nil)
```

### 10.3 Composition

```go
// Embedding interfaces for composition
type Logger interface {
    Log(msg string)
}

type MetricsCollector interface {
    Record(name string, value float64)
}

type Observability interface {
    Logger
    MetricsCollector
}

// Struct embedding
type Server struct {
    http.Server
    logger Logger
}
```

## 11. Unit Tests

### 11.1 Structure

Go has built-in testing with the `testing` package. Tests live in `*_test.go` files alongside the code they test.

```go
package user

import (
    "testing"
)

func TestValidateEmail(t *testing.T) {
    u := User{Email: "test@example.com"}
    if err := u.Validate(); err != nil {
        t.Errorf("expected no error, got %v", err)
    }
}

func TestInvalidEmail(t *testing.T) {
    u := User{Email: "invalid"}
    if err := u.Validate(); err == nil {
        t.Error("expected error for invalid email")
    }
}
```

### 11.2 Table-Driven Tests

```go
func TestDivide(t *testing.T) {
    tests := []struct {
        name     string
        a, b     float64
        expected float64
        wantErr  bool
    }{
        {name: "positive", a: 10, b: 2, expected: 5, wantErr: false},
        {name: "division by zero", a: 10, b: 0, expected: 0, wantErr: true},
        {name: "negative", a: -6, b: 3, expected: -2, wantErr: false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Divide(tt.a, tt.b)
            if (err != nil) != tt.wantErr {
                t.Errorf("Divide() error = %v, wantErr %v", err, tt.wantErr)
            }
            if result != tt.expected {
                t.Errorf("Divide() = %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### 11.3 Assertions

Go's standard `testing` package uses `t.Error`/`t.Fatal`. For richer assertions, use testify:

```go
import "github.com/stretchr/testify/assert"

func TestWithAssertions(t *testing.T) {
    result, err := Divide(10, 2)
    assert.NoError(t, err)
    assert.Equal(t, 5.0, result)
    assert.IsType(t, float64(0), result)
}
```

### 11.4 Commands

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific test
go test -run TestDivide/positive -v

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run with race detection
go test -race ./...

# Run with short mode (skip integration tests)
go test -short ./...
```

## 12. Mocks and Testability

### 12.1 Mock Strategies

Use interfaces to enable mocking. Prefer handwritten mocks for small interfaces and testify/mock for larger ones.

```go
type UserRepository interface {
    FindByID(id int64) (*User, error)
    Save(u *User) error
}

// testify mock implementation
type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) FindByID(id int64) (*User, error) {
    args := m.Called(id)
    return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserRepository) Save(u *User) error {
    args := m.Called(u)
    return args.Error(0)
}
```

### 12.2 Dependency Injection

Pass dependencies explicitly via constructor functions.

```go
type Service struct {
    repo UserRepository
    log  *slog.Logger
}

func NewService(repo UserRepository, log *slog.Logger) *Service {
    return &Service{repo: repo, log: log}
}

// Usage in tests
func TestService_GetUser(t *testing.T) {
    mockRepo := new(MockUserRepository)
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    svc := NewService(mockRepo, logger)

    mockRepo.On("FindByID", int64(1)).Return(&User{ID: 1, Name: "Alice"}, nil)
    user, err := svc.GetUser(1)
    assert.NoError(t, err)
    assert.Equal(t, "Alice", user.Name)
    mockRepo.AssertExpectations(t)
}
```

### 12.3 Test Doubles

- **Stubs**: Return fixed values for testing
- **Mocks**: Record expectations and verify interactions
- **Fakes**: Lightweight implementations (e.g., in-memory database)
- **Spies**: Record calls for later inspection

```go
type SpyLogger struct {
    messages []string
}

func (s *SpyLogger) Log(msg string) {
    s.messages = append(s.messages, msg)
}
```

## 13. Integration Tests

### 13.1 Structure and Organization

Use build tags to separate integration tests from unit tests:

```go
// File: repository_test.go
//go:build integration

package repository

import (
    "testing"
)

func TestPostgresRepository(t *testing.T) {
    // requires running PostgreSQL
}
```

### 13.2 Selective Execution

```bash
# Run only unit tests
go test -short ./...

# Run integration tests
go test -tags=integration ./...

# Run all tests
go test -tags=integration ./... -count=1
```

### 13.3 Real Dependencies

Use testcontainers-go for managing real dependencies in integration tests:

```go
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()
    // testcontainers-go provides PostgreSQL containers
    connStr := os.Getenv("TEST_DATABASE_URL")
    db, err := sql.Open("pgx", connStr)
    if err != nil {
        t.Fatalf("failed to connect: %v", err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}
```

## 14. Load and Stress Tests

### 14.1 Tools

- **hey**: HTTP load generator
- **vegeta**: HTTP load testing tool with reporting
- **Go test + benchmarks**: For function-level load testing

### 14.2 Load Benchmarks

```go
func BenchmarkHandler(b *testing.B) {
    handler := NewHandler()
    req := httptest.NewRequest("GET", "/api/users", nil)
    w := httptest.NewRecorder()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        handler.ServeHTTP(w, req)
    }
}
```

### 14.3 Concurrency Tests

```go
func TestConcurrentAccess(t *testing.T) {
    counter := NewCounter()
    var wg sync.WaitGroup

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            counter.Increment()
        }()
    }
    wg.Wait()

    assert.Equal(t, int64(100), counter.Value())
}
```

## 15. Profiling and Diagnostics

### 15.1 CPU and Memory Profiling

Go has built-in profiling via `pprof`:

```go
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    // application code
}
```

### 15.2 Diagnostic Tools

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench=.

# Memory profile
go test -memprofile=mem.prof -bench=.

# Mutex profile
go test -mutexprofile=mutex.prof -bench=.

# Analyze profiles
go tool pprof -http=:8080 cpu.prof
go tool pprof -http=:8080 mem.prof
```

### 15.3 Performance Analysis

- `go tool trace`: Analyze goroutine scheduling and latency
- `GODEBUG=gctrace=1`: Print GC cycles to stderr
- `runtime.ReadMemStats`: Programmatic memory inspection
- `go test -benchmem`: Include allocation statistics in benchmarks
- `go test -trace=trace.out`: Execution tracing

```bash
go test -trace=trace.out ./...
go tool trace trace.out
```

## 16. Benchmarks

### 16.1 Writing Benchmarks

Benchmark functions start with `Benchmark` and accept `*testing.B`:

```go
func BenchmarkSum(b *testing.B) {
    nums := make([]int, 1000)
    for i := range nums {
        nums[i] = i
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        Sum(nums)
    }
}

// Benchmark different approaches
func BenchmarkSumRange(b *testing.B) {
    nums := make([]int, 1000)
    for i := range nums {
        nums[i] = i
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        var sum int
        for _, n := range nums {
            sum += n
        }
    }
}
```

### 16.2 Sub-benchmarks

```go
func BenchmarkSort(b *testing.B) {
    sizes := []int{10, 100, 1000, 10000}
    for _, size := range sizes {
        b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
            data := randomSlice(size)
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                Sort(data)
            }
        })
    }
}
```

### 16.3 Execution and Analysis

```bash
# Run benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkSum -benchtime=5s

# Compare benchmarks (install benchstat)
go test -bench=. -count=5 > old.txt
# after changes
go test -bench=. -count=5 > new.txt
benchstat old.txt new.txt

# Bench with allocations
go test -bench=. -benchmem ./...

## 17. Optimization

### 17.1 Principles

- Measure before optimizing: use profiles and benchmarks
- Focus on algorithmic complexity first
- Document performance trade-offs in code comments
- Premature optimization is the root of all evil

### 17.2 Common Optimizations

```go
// Pre-allocate slices when size is known
// Bad: append without capacity
var users []User
for _, u := range source {
    users = append(users, u)
}

// Good: pre-allocate capacity
users := make([]User, 0, len(source))
for _, u := range source {
    users = append(users, u)
}

// Use strings.Builder for string concatenation
var b strings.Builder
for _, s := range parts {
    b.WriteString(s)
}
result := b.String()
```

### 17.3 Memory Optimization

```go
// Escape analysis: prefer stack allocation
// This escapes to heap (bad for hot paths)
func NewUser() *User {
    return &User{Name: "Alice"}
}

// Pass by value when struct is small (<= 4 words)
func Process(u User) string {
    return u.Name
}

// Reuse buffers
var bufferPool = sync.Pool{
    New: func() any {
        return make([]byte, 4096)
    },
}

buf := bufferPool.Get().([]byte)
defer bufferPool.Put(buf)
```

### 17.4 Basic Performance

- Use `map[int]struct{}` instead of `map[int]bool` for sets (zero memory for value)
- Avoid `fmt.Sprintf` in hot paths; use `strconv` for number formatting
- Use `sync.Pool` for frequently allocated temporary objects
- Benchmark with `-benchmem` to track allocation counts

## 18. Security

### 18.1 Essential Practices

- Never hardcode secrets: use environment variables or secret managers
- Validate all external input: request params, file uploads, API payloads
- Use HTTPS in all communications
- Apply rate limiting to public endpoints
- Keep dependencies updated: run `go vet` and `govulncheck`
- Principle of least privilege for database users and service accounts

```go
// Input validation
func CreateUser(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Email string `json:"email"`
    }
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }
    if !strings.Contains(input.Email, "@") {
        http.Error(w, "invalid email", http.StatusBadRequest)
        return
    }
}
```

### 18.2 Tools

```bash
# Official Go vulnerability scanner
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# Static analysis with security focus
golangci-lint run --enable gosec ./...
```

### 18.3 Security at API Boundaries

- Defensive copying: never return internal slices or maps directly
- Sanitize SQL with parameterized queries (NEVER string concatenation)
- Set HTTP security headers (Content-Security-Policy, X-Frame-Options)
- Use `http.TimeoutHandler` to prevent slow-loris attacks

## 19. Code Patterns

### 19.1 Early Return

```go
// Good: Early return reduces nesting
func processOrder(o Order) error {
    if !o.IsValid() {
        return ErrInvalidOrder
    }
    if !o.IsPaid() {
        return ErrOrderNotPaid
    }
    // process order
    return nil
}

// Bad: Deep nesting
func processOrder(o Order) error {
    if o.IsValid() {
        if o.IsPaid() {
            // process order
            return nil
        }
    }
    return errors.New("invalid")
}
```

### 19.2 Separation of Concerns

```go
// Handler: HTTP concerns only
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
    user, err := h.service.GetUser(r.Context(), id)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(user)
}

// Service: Business logic only
func (s *UserService) GetUser(ctx context.Context, id int64) (*User, error) {
    return s.repo.FindByID(ctx, id)
}

// Repository: Data access only
func (r *UserRepository) FindByID(ctx context.Context, id int64) (*User, error) {
    // database query
}
```

### 19.3 DRY

```go
// Extract repeated validation into helper
func validatePagination(page, size int) error {
    if page < 1 {
        return fmt.Errorf("page must be >= 1: %d", page)
    }
    if size < 1 || size > 100 {
        return fmt.Errorf("size must be 1-100: %d", size)
    }
    return nil
}
```

### 19.4 Variable Scope

```go
// Good: narrow scope
if err := process(); err != nil {
    return err
}

// Bad: broader scope than needed
var err error
// ... many lines
if err = process(); err != nil {
    return err
}
```

## 20. Dependency Management

### 20.1 Principles

- Prefer standard library over third-party dependencies
- Use well-maintained, widely adopted packages
- Minimize dependency count: each dep adds attack surface and maintenance burden
- Pin to explicit versions, not ranges

### 20.2 Commands

```bash
# Audit dependencies for vulnerabilities
govulncheck ./...

# Show dependency graph
go mod graph

# Show outdated dependencies
go list -u -m all

# Remove unused deps
go mod tidy

# Clean module cache
go clean -modcache
```

## 21. Comments and Documentation

### 21.1 Code Comments

Comments explain "why", not "what". Code should be self-documenting.

```go
// Good: Explains reasoning (why, not what)
// Use batch processing to avoid OOM with large datasets
const batchSize = 100

// Bad: states the obvious
// set batch size to 100
const batchSize = 100
```

### 21.2 API Documentation

Doc comments appear before exported declarations. Follow Go's doc comment syntax.

```go
// Package user provides user management functionality.
package user

// User represents a registered user in the system.
type User struct {
    ID    int64
    Name  string
    Email string
}

// Validate checks user fields and returns validation errors.
func (u *User) Validate() error {
    // implementation
}
```

### 21.3 Package Documentation

Every package should have a package comment explaining its purpose.

```go
/*
Package handler provides HTTP handlers for the user management API.

Handlers are grouped by resource and use chi router for routing.
All handlers accept and return JSON.
*/
package handler
```

## 22. Database

### 22.1 Approach

Go offers three levels of database interaction:
- **Standard `database/sql`**: Use `pgx` as a driver for PostgreSQL
- **Query builder**: `sq` or `squirrel` for dynamic queries
- **Code generation**: `sqlc` for type-safe generated code

Choose based on project complexity. Start with `database/sql` + `pgx` for simplicity.

### 22.2 Connection and Driver

```go
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
)

func NewDB(dsn string) (*sql.DB, error) {
    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return nil, fmt.Errorf("open database: %w", err)
    }

    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)

    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("ping database: %w", err)
    }
    return db, nil
}
```

```go
// Safe query with parameterized statements
func GetUserByEmail(db *sql.DB, email string) (*User, error) {
    query := "SELECT id, name, email FROM users WHERE email = $1"

    var u User
    err := db.QueryRow(query, email).Scan(&u.ID, &u.Name, &u.Email)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("query user by email: %w", err)
    }
    return &u, nil
}
```

### 22.3 Migrations

Use `golang-migrate/migrate` or write raw SQL migration files:

```bash
# Install migration tool
go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Create migration
migrate create -ext sql -dir migrations create_users

# Run migrations
migrate -path migrations -database "$DATABASE_URL" up

# Rollback
migrate -path migrations -database "$DATABASE_URL" down 1
```

```sql
-- migrations/20240101000001_create_users.up.sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 22.4 Best Practices

- ALWAYS use parameterized queries (`$1`, `$2`) to prevent SQL injection
- Add appropriate indexes for frequent query patterns
- Use connection pooling (configure `SetMaxOpenConns`, `SetMaxIdleConns`)
- Wrap transactions explicitly with `Begin`/`Commit`/`Rollback`
- Handle connection errors with retries and backoff

```go
// Transaction pattern
func CreateUserTx(db *sql.DB, u *User) error {
    tx, err := db.Begin()
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback() // no-op if committed

    _, err = tx.Exec(
        "INSERT INTO users (name, email) VALUES ($1, $2)",
        u.Name, u.Email,
    )
    if err != nil {
        return fmt.Errorf("insert user: %w", err)
    }

    return tx.Commit()
}
```

## 23. Logs and Observability

### 23.1 Log Levels

Go 1.21+ provides structured logging via `log/slog` with levels: `DEBUG`, `INFO`, `WARN`, `ERROR`.

```go
import "log/slog"

// Configure structured JSON logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
slog.SetDefault(logger)
```

### 23.2 Structured Logs

```go
// Good: Structured fields for machine parsing
slog.Info("user created",
    "user_id", user.ID,
    "email", user.Email,
    "duration_ms", elapsed.Milliseconds(),
)

// Bad: Unstructured, hard to parse
slog.Info(fmt.Sprintf("user %d created with email %s", user.ID, user.Email))
```

### 23.3 Logging Implementation

```go
// Logger middleware example
func LoggerMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        slog.Info("request started",
            "method", r.Method,
            "path", r.URL.Path,
            "remote_addr", r.RemoteAddr,
        )

        next.ServeHTTP(w, r)

        slog.Info("request completed",
            "method", r.Method,
            "path", r.URL.Path,
            "duration_ms", time.Since(start).Milliseconds(),
        )
    })
}
```

### 23.4 Metrics and Observability

- Expose metrics endpoints (`/health`, `/metrics`)
- Use `expvar` or `prometheus` client for application metrics
- Instrument I/O operations (database queries, HTTP calls, message queue operations)
- Keep label cardinality low

```go
// Health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
    })
}

// Integrate into chi
r := chi.NewRouter()
r.Get("/health", healthHandler)
r.Get("/metrics", telemetry.Handler)
```

## 24. Golden Rules

1. **Simplicity**: Write straightforward code. Prefer a simple loop over a complex chain of abstractions.
2. **Explicit errors**: Never ignore errors. Handle them, wrap them, or propagate them with context.
3. **Tests first**: Table-driven tests ensure edge cases are covered. Run `go test -race` regularly.
4. **Document interfaces**: Doc comments on all exported names. Package comments explain the "why".
5. **Measure performance**: Use benchmarks, pprof, and tracing before optimizing. Trust data over intuition.
6. **Format automatically**: Run `gofmt` and `goimports` on every save. Let the machine enforce style.

## 25. Pre-Commit Checklist

### Code
- [ ] `gofmt -s -w .` applied
- [ ] `go vet ./...` passes without errors
- [ ] `golangci-lint run ./...` clean
- [ ] `go build ./...` compiles successfully

### Tests
- [ ] `go test -race ./...` all pass
- [ ] Coverage >= 70% on critical paths: `go test -coverprofile=out ./...`
- [ ] Integration tests pass: `go test -tags=integration ./...`
- [ ] Benchmarks validated if performance-critical code changed

### Quality
- [ ] All errors handled explicitly (no `_ = fn()` or unchecked returns)
- [ ] Resources properly closed (defer `db.Close()`, response.Body.Close())
- [ ] No hardcoded secrets, API keys, or passwords
- [ ] `govulncheck ./...` shows no vulnerabilities

### Documentation
- [ ] Exported functions/types have doc comments
- [ ] README updated for any API or behavioral changes
- [ ] Comments explain "why", not "what"

### Docker
- [ ] `docker compose build` succeeds
- [ ] `docker compose up` starts without errors
- [ ] Health check endpoint returns 200

## 26. References

### Official Documentation
- Go Language Specification: https://go.dev/ref/spec
- Effective Go: https://go.dev/doc/effective_go
- Go Doc Comments: https://go.dev/doc/comment
- How to Write Go Code: https://go.dev/doc/code
- Organizing a Go Module: https://go.dev/doc/modules/layout
- Go Modules Reference: https://go.dev/ref/mod

### Style Guides
- Google Go Style Guide: https://google.github.io/styleguide/go/guide
- Google Go Style Decisions: https://google.github.io/styleguide/go/decisions
- Google Go Best Practices: https://google.github.io/styleguide/go/best-practices
- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments

### Essential Tools
- gofmt: https://go.dev/cmd/gofmt
- staticcheck: https://staticcheck.io
- golangci-lint: https://golangci-lint.run
- govulncheck: https://go.dev/blog/vulncheck

### Testing and Performance
- testing package: https://pkg.go.dev/testing
- pprof: https://go.dev/doc/diagnostics#pprof
- trace: https://go.dev/doc/diagnostics#tracing
- benchstat: https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
```
