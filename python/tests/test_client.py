import json
import unittest

import httpx

from gregal_client import Client, ProtocolError


class ClientTests(unittest.IsolatedAsyncioTestCase):
    async def test_session_lifecycle_and_mode_use_backend_contract(self):
        requests = []

        def handle(request):
            requests.append(request)
            if request.url.path.endswith("/open"):
                return httpx.Response(200, json={"id": "py-contract", "title": "Contract"})
            if request.url.path.endswith("/close"):
                return httpx.Response(200, json={"ok": True, "closed": "py-contract"})
            return httpx.Response(200, json={"mode": "inspect"})

        async with Client("http://backend", "employee-token", transport=httpx.MockTransport(handle)) as client:
            opened = await client.open_session(session="py-contract", title="Contract")
            mode = await client.set_mode("py-contract", "inspect")
            closed = await client.close_session("py-contract")

        self.assertEqual(opened["id"], "py-contract")
        self.assertEqual(mode["mode"], "inspect")
        self.assertEqual(closed["closed"], "py-contract")
        self.assertEqual([request.url.path for request in requests], [
            "/api/sessions/open", "/api/mode", "/api/sessions/close",
        ])
        self.assertEqual(json.loads(requests[0].content), {"id": "py-contract", "title": "Contract"})
        self.assertEqual(requests[1].headers["X-Gregal-Session"], "py-contract")
        self.assertEqual(json.loads(requests[1].content), {"mode": "inspect"})
        self.assertEqual(json.loads(requests[2].content), {"id": "py-contract"})
        for request in requests:
            self.assertEqual(request.headers["Authorization"], "Bearer employee-token")

    async def test_event_poll_uses_session_cursor_and_limit(self):
        requests = []

        def handle(request):
            requests.append(request)
            return httpx.Response(200, json={"events": [], "cursor": 9, "next": 9})

        async with Client("http://backend", transport=httpx.MockTransport(handle)) as client:
            page = await client.events("session-2", after=9, limit=25)

        self.assertEqual(page["next"], 9)
        request = requests[0]
        self.assertEqual(request.url.path, "/api/v2/events")
        self.assertEqual(dict(request.url.params), {"session": "session-2", "after": "9", "limit": "25"})
        self.assertEqual(request.headers["X-Gregal-Session"], "session-2")

    async def test_empty_event_page_normalizes_go_null_slice(self):
        async with Client(
            "http://backend",
            transport=httpx.MockTransport(lambda _: httpx.Response(200, json={"events": None, "cursor": 0, "next": 0})),
        ) as client:
            page = await client.events("empty-session")
        self.assertEqual(page["events"], [])

    async def test_event_page_rejects_non_list_events(self):
        async with Client(
            "http://backend",
            transport=httpx.MockTransport(lambda _: httpx.Response(200, json={"events": {"id": 1}})),
        ) as client:
            with self.assertRaises(ProtocolError):
                await client.events("session")

    async def test_submission_and_interactions_keep_scope(self):
        requests = []

        def handle(request):
            requests.append(request)
            return httpx.Response(200, json={"run": {"id": 7}, "cursor": 12})

        async with Client("http://backend", "employee-token", transport=httpx.MockTransport(handle)) as client:
            result = await client.submit("employee-session", "Analyze data", idempotency_key="turn-1")
            self.assertEqual(result["cursor"], 12)
            await client.run("employee-session", 7)
            await client.cancel("employee-session", 7)
            await client.approve("employee-session", "permission", False)
            await client.answer("employee-session", "question", "Use approved data")
        self.assertEqual(len(requests), 5)
        for request in requests:
            self.assertEqual(request.headers["Authorization"], "Bearer employee-token")
            self.assertEqual(request.headers["X-Gregal-Session"], "employee-session")
        self.assertIn(b'"idempotency_key":"turn-1"', requests[0].content)

    async def test_stream_preserves_payload_and_skips_old_events(self):
        def handle(request):
            self.assertEqual(request.headers["Last-Event-ID"], "4")
            self.assertEqual(dict(request.url.params), {"session": "session", "after": "4"})
            return httpx.Response(200, headers={"Content-Type": "text/event-stream"}, text=(
                ': heartbeat\n\n'
                'id: 4\nevent: text\ndata: {"id":4,"kind":"text"}\n\n'
                'id: 5\nevent: approve_request\ndata: {"id":5,"kind":"approve_request","payload":{"key":"permission"}}\n\n'
            ))

        async with Client("http://backend", transport=httpx.MockTransport(handle)) as client:
            events = [event async for event in client.stream("session", after=4)]
        self.assertEqual(events, [{"id": 5, "kind": "approve_request", "payload": {"key": "permission"}}])

    async def test_rejects_incompatible_backend_and_invalid_mode(self):
        async with Client("http://backend", transport=httpx.MockTransport(lambda _: httpx.Response(200, json=[]))) as client:
            with self.assertRaises(ProtocolError):
                await client.health()
            with self.assertRaises(ValueError):
                await client.set_mode("session", "unrestricted")

    async def test_rejects_health_with_malformed_capabilities(self):
        body = {"status": "ok", "protocol": "gregal.v1", "instance_id": "instance", "capabilities": []}
        async with Client("http://backend", transport=httpx.MockTransport(lambda _: httpx.Response(200, json=body))) as client:
            with self.assertRaises(ProtocolError):
                await client.health()

    async def test_authentication_errors_propagate(self):
        async with Client("http://backend", transport=httpx.MockTransport(lambda _: httpx.Response(401))) as client:
            with self.assertRaises(httpx.HTTPStatusError):
                await client.health()


if __name__ == "__main__":
    unittest.main()
