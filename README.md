# go-rate-limiter
A high-performance rate limiting service built to handle distributed traffic. 

## Implementation Decisions
- **Sliding Window Log Algorithm:** Implemented via Redis **Sorted Sets (ZSETs)** to provide 100% precision, eliminating the "double-window" burst vulnerability found in Fixed Window counters.
- **Atomic Operations:** Utilized **Lua Scripting** to execute complex cleanup-and-count logic in a single Redis transaction, preventing race conditions and reducing network round-trips.
- **Observability:** Integrated **Prometheus** metrics for real-time monitoring of allowed/blocked traffic, visualized through a pre-configured **Grafana** dashboard.
- **Production Readiness:** Features **Graceful Shutdown** handling and **Dependency Injection** for testability.

## Tech Stack
- **Go 1.25.5**: Focused on concurrency.
- **Redis**: Primary data store for distributed state.
- **Docker & Compose**: Full containerization for both the app and monitoring stack.
- **Prometheus/Grafana**: Full observability suite.

## Getting Started
1. `docker compose up --build`
2. Access the API at `localhost:8080`
3. View metrics at `localhost:3000` (admin/admin)

## Further Work
- Track access requests by user ID / token, rather than IP.
- Return rate limit, requests left, and window remaining to user.