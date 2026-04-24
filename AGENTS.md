This is a Go backend project called minilog.

Structure:
- main.go starts the server
- internal/api contains HTTP handlers
- internal/logstore contains in-memory storage
- internal/model defines data structures

Rules:
- Keep code simple
- Use standard library only
- Separate HTTP and storage logic
- Validate all inputs
- Add tests for new behavior
- Do not add external dependencies