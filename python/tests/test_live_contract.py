import os
import unittest
import uuid

from gregal_client import Client


BASE_URL = os.environ.get("GREGAL_CONTRACT_URL")
TOKEN = os.environ.get("GREGAL_CONTRACT_TOKEN", "")


@unittest.skipUnless(BASE_URL, "set GREGAL_CONTRACT_URL to run the live backend contract check")
class LiveContractTests(unittest.IsolatedAsyncioTestCase):
    async def test_health_session_events_and_cleanup(self):
        requested_session = "py-contract-" + uuid.uuid4().hex[:16]
        async with Client(BASE_URL, TOKEN) as client:
            health = await client.health()
            self.assertTrue(health["instance_id"])

            opened = await client.open_session(session=requested_session, title="Python contract check")
            session = opened.get("id", "")
            try:
                self.assertTrue(session)
                self.assertTrue(session.endswith(requested_session))
                page = await client.events(session, after=0, limit=1)
                self.assertIsInstance(page["events"], list)
                self.assertGreaterEqual(page["cursor"], 0)
                self.assertGreaterEqual(page["next"], page["cursor"])
            finally:
                if session:
                    closed = await client.close_session(session)
                    self.assertEqual(closed["closed"], session)


if __name__ == "__main__":
    unittest.main()
