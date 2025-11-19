# Collaboration stack and integrations porting status

The requested migration away from the existing Python Socket.IO + Yjs stack and the related background job and adapter plumbing has **not** been implemented in this iteration. The current codebase still relies on `python-socketio` and `pycrdt` in `backend/open_webui/socket/main.py`, plus the existing Redis-backed pools, periodic tasks, and router adapters.

## Scope summary
- Replacing the collaboration stack with an alternative implementation (e.g., `rust-socketio` or `go-socket.io`) requires a full rewrite of `backend/open_webui/socket/main.py` and the supporting `open_webui/socket/utils.py` utilities.
- Periodic/background jobs from `open_webui/tasks.py` remain bound to the current async task helpers and Redis usage cleanup flow.
- Adapters for the Ollama/OpenAI/image/audio/pipeline integrations in `open_webui/routers` and `open_webui/functions.py` are unchanged and continue to follow the existing `open_webui/config.py` environment toggles.
- Redis usage (sessions, rate limits, websocket pools) along with `RedisLock` semantics remain tied to the current Redis helper layer.

## Next steps
1. Introduce a new socket server implementation and rebind event handlers to maintain collaborative editing and usage tracking.
2. Port or recreate the scheduled jobs, ensuring usage cleanup, sync tasks, and async workflows are registered under the new runtime.
3. Rebuild adapter layers for model, image, and audio integrations so they reflect the configuration contract in `open_webui/config.py`.
4. Audit and reimplement Redis-backed state (sessions, pools, locks) to ensure compatibility with the new stack.

These steps are intentionally documented here to outline the remaining migration work and avoid silent regressions.
