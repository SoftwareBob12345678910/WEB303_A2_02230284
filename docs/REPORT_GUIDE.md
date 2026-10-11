# Report guide (VLE submission)

Every screenshot needs a **caption** ("Figure N: …") and **2–3 sentences** saying what it shows and which requirement it proves.

## Suggested structure
1. Introduction — domain, why these two resources, business rules
2. Architecture — Mermaid/diagram from the README; explain bounded contexts and Database per Service
3. Service B (Telescope) — proto, schema, validation rules, gRPC error codes
4. Service A (Session) — proto, schema, how it calls B, business rules
5. API Gateway — routes table, gRPC→HTTP mapping table
6. Resilience — timeout / retry / breaker settings and why you chose them
7. Deployment — multi-stage Dockerfile explanation, compose, env vars
8. Testing evidence — screenshots below
9. Conclusion / limitations

## Screenshot checklist

| # | Screenshot | Proves |
|---|---|---|
| 1 | `docker compose up --build` finishing; `docker compose ps` with 5 healthy containers | one-command deployment |
| 2 | `docker images` showing small final images | multi-stage builds |
| 3 | `docker compose exec telescope-db psql -U telescope_user -d telescopes_db -c '\dt'` and the same for session-db | separate databases |
| 4 | Postman: Telescopes create / list / get / update | Req 1 |
| 5 | Postman: Sessions create / list / get / update | Req 2 |
| 6 | Session create response showing `telescope_details_available: true` | A → B gRPC call |
| 7 | Unknown telescope → 404; maintenance telescope → 409; double booking → 409 | business rules + error mapping |
| 8 | Invalid payloads → 400, then `SELECT count(*)` showing nothing was stored | Req 6 validation |
| 9 | `docker compose stop telescope-service`; POST /sessions → 503 | resilience: meaningful error |
| 10 | `docker compose logs session-service` showing "retrying…" lines | retry with backoff |
| 11 | Logs: `circuit breaker "telescope-service": closed -> open` and instant 503s | circuit breaker |
| 12 | GET /sessions/{id} while B is down → 200 with `telescope_details_available: false` | fallback |
| 13 | `docker compose start telescope-service`, wait 10 s, booking works; logs `half-open -> closed` | recovery |
| 14 | Delete session / telescope → 204, then GET → 404 | CRUD completeness |
| 15 | `go test ./...` passing | code quality |
| 16 | Repo tree, `.env`, Dockerfile, Postman collection runner results | documentation + config |
