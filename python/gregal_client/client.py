"""Async transport for the same sessions, runs and events used by native clients."""

from collections.abc import AsyncIterator
from typing import Any

import httpx

PROTOCOL = "gregal.v1"
MODES = frozenset({"chat", "inspect", "code", "goal", "autonomous"})


class ProtocolError(RuntimeError):
    pass


class Client:
    """One authenticated backend principal; pass a session on every operation.

    The token is a Gregal user token, never an LLM provider credential. Create
    separate clients for separate employees. An injected transport supports
    deterministic tests without contacting a model or corporate service.
    """

    def __init__(self, base_url: str, token: str = "", *, transport=None):
        self._http = httpx.AsyncClient(
            base_url=base_url.rstrip("/") + "/",
            headers={"Authorization": f"Bearer {token}"} if token else {},
            transport=transport,
            timeout=httpx.Timeout(30.0, read=30.0),
            follow_redirects=False,
        )

    async def __aenter__(self):
        return self

    async def __aexit__(self, *_):
        await self.close()

    async def close(self):
        await self._http.aclose()

    async def _json(self, method: str, route: str, *, session="", **kwargs):
        headers = {"X-Gregal-Session": session} if session else {}
        response = await self._http.request(method, route, headers=headers, **kwargs)
        response.raise_for_status()
        return response.json()

    async def health(self) -> dict[str, Any]:
        health = await self._json("GET", "api/health")
        if not isinstance(health, dict):
            raise ProtocolError("Incompatible Gregal backend")
        caps = health.get("capabilities", {})
        if health.get("status") != "ok" or health.get("protocol") != PROTOCOL:
            raise ProtocolError("Incompatible Gregal backend")
        if not isinstance(caps, dict) or not health.get("instance_id") or not all(
            caps.get(key) for key in ("runs", "durable_events", "interactive_events")
        ) or not (caps.get("events") or caps.get("event_stream")):
            raise ProtocolError("The backend does not support the interactive v2 contract")
        return health

    async def open_session(self, *, title="", session="") -> dict[str, Any]:
        return await self._json("POST", "api/sessions/open", json={"id": session, "title": title})

    async def close_session(self, session: str) -> dict[str, Any]:
        if not session:
            raise ValueError("A session is required")
        return await self._json("POST", "api/sessions/close", json={"id": session})

    async def set_mode(self, session: str, mode: str):
        if mode not in MODES:
            raise ValueError(f"Unknown mode: {mode}")
        return await self._json("POST", "api/mode", session=session, json={"mode": mode})

    async def submit(self, session: str, task: str, *, mode="", idempotency_key="", images=None):
        """Return both run and cursor. Reuse the key after a failed network reply.

        Set the session mode explicitly with set_mode(). A per-turn mode only
        restricts existing privileges; it cannot silently upgrade chat to code.
        """
        if not session or not task.strip():
            raise ValueError("A session and a nonempty task are required")
        if mode and mode not in MODES:
            raise ValueError(f"Unknown mode: {mode}")
        return await self._json(
            "POST", "api/v2/runs", session=session,
            json={"task": task, "mode": mode, "idempotency_key": idempotency_key, "images": images or []},
        )

    async def run(self, session: str, run_id: int):
        return (await self._json("GET", f"api/v2/runs/{run_id}", session=session))["run"]

    async def cancel(self, session: str, run_id: int):
        return await self._json("POST", f"api/v2/runs/{run_id}/cancel", session=session, json={})

    async def events(self, session: str, *, after=0, limit=100) -> dict[str, Any]:
        page = await self._json(
            "GET", "api/v2/events", session=session,
            params={"session": session, "after": after, "limit": limit},
        )
        if not isinstance(page, dict) or "events" not in page:
            raise ProtocolError("Invalid events response")
        if page["events"] is None:
            page["events"] = []
        elif not isinstance(page["events"], list):
            raise ProtocolError("Invalid events response")
        return page

    async def stream(self, session: str, *, after=0) -> AsyncIterator[dict[str, Any]]:
        """Yield durable events including structured approval/question payloads.

        Persist each consumed event's id. On reconnect pass that id as after;
        never submit a second run just because the stream disconnected.
        """
        import json

        async with self._http.stream(
            "GET", "api/v2/events/stream",
            params={"session": session, "after": after},
            headers={"X-Gregal-Session": session, "Last-Event-ID": str(after)},
            timeout=httpx.Timeout(30.0, read=None),
        ) as response:
            response.raise_for_status()
            if "text/event-stream" not in response.headers.get("content-type", ""):
                raise ProtocolError("Expected an SSE response")
            data = []
            async for line in response.aiter_lines():
                if line == "":
                    if data:
                        event = json.loads("\n".join(data))
                        data.clear()
                        if not isinstance(event, dict) or not isinstance(event.get("id"), int):
                            raise ProtocolError("Event has no durable cursor")
                        if event["id"] > after:
                            after = event["id"]
                            yield event
                    continue
                if line.startswith("data:"):
                    data.append(line[5:].removeprefix(" "))

    async def approve(self, session: str, key: str, approve: bool, *, remember=False):
        return await self._json(
            "POST", "api/approve", session=session,
            json={"key": key, "approve": approve, "remember": remember},
        )

    async def answer(self, session: str, key: str, answer: str):
        return await self._json("POST", "api/question", session=session, json={"key": key, "answer": answer})
