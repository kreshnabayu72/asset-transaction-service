# Asset Transaction API

Open this directory as a collection in Bruno and select the `local` environment.
Start the service with `go run ./main.go` or `docker compose up -d --build`.
Run **Health** to check `GET /health`; it returns `200` with `{"status":"ok"}`.

Change `baseUrl` in the environment when using another port or the gateway.
The health endpoint is the only implemented API; domain requests will be added
alongside their implementations.
