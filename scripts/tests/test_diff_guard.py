"""Tests for the diff guard. Run: uv run python -m unittest discover -s scripts/tests"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from diff_guard import MAX_CODE_LINES, Change, evaluate, parse_diff

DIFF = """\
diff --git src/app.py src/app.py
index 1111111..2222222 100644
--- src/app.py
+++ src/app.py
@@ -3 +3,2 @@ def run():
-    return old()
+    value = new()
+    return value
@@ -9 +10 @@
--- a removed sql comment
+++ an added line that looks like a header
diff --git src/gone.py src/gone.py
deleted file mode 100644
--- src/gone.py
+++ /dev/null
@@ -1 +0,0 @@
-x = 1
diff --git src/fresh.py src/fresh.py
new file mode 100644
--- /dev/null
+++ src/fresh.py
@@ -0,0 +1 @@
+y = 2
"""


def rules(changes: dict[str, Change]) -> list[str]:
    return [finding.rule for finding in evaluate(changes)[0]]


class ParseDiffTest(unittest.TestCase):
    def test_splits_added_and_removed_lines_per_file(self):
        changes = parse_diff(DIFF)
        self.assertEqual(
            changes["src/app.py"].added,
            ["    value = new()", "    return value", "++ an added line that looks like a header"],
        )
        self.assertEqual(
            changes["src/app.py"].removed, ["    return old()", "-- a removed sql comment"]
        )

    def test_tracks_new_and_deleted_files(self):
        changes = parse_diff(DIFF)
        self.assertTrue(changes["src/fresh.py"].new)
        self.assertFalse(changes["src/app.py"].new)
        self.assertEqual(changes["src/gone.py"].removed, ["x = 1"])


class EvaluateTest(unittest.TestCase):
    def test_clean_change_has_no_findings(self):
        self.assertEqual(rules({"src/app.py": Change(added=["x = compute()"])}), [])

    def test_flags_new_suppressions_in_code(self):
        for line in ("x = f()  # noqa: E501", "y = g()  # type: ignore", "// @ts-ignore"):
            with self.subTest(line=line):
                self.assertEqual(rules({"src/a.ts": Change(added=[line])}), ["suppressions"])

    def test_ignores_suppression_text_outside_code_files(self):
        self.assertEqual(rules({"docs/guide.md": Change(added=["never add # noqa"])}), [])

    def test_does_not_flag_suppressions_that_were_already_there(self):
        self.assertEqual(rules({"src/a.py": Change(removed=["x = f()  # noqa"])}), [])

    def test_flags_skipped_and_focused_tests(self):
        for line in ("@pytest.mark.skip", "it.only('works', () => {})", "xdescribe('x', f)"):
            with self.subTest(line=line):
                change = Change(added=[line])
                self.assertEqual(rules({"tests/test_a.py": change}), ["skipped-tests"])

    def test_flags_net_removal_of_tests(self):
        change = Change(removed=["def test_one():", "def test_two():"], added=["def test_one():"])
        self.assertEqual(rules({"tests/test_a.py": change}), ["tests-removed"])

    def test_allows_moving_a_test_to_another_file(self):
        changes = {
            "tests/test_a.py": Change(removed=["def test_one():"]),
            "tests/test_b.py": Change(added=["def test_one():"], new=True),
        }
        self.assertEqual(rules(changes), [])

    def test_flags_junk_files_only_when_new(self):
        for path in ("src/parser_v2.py", "src/utils.old.ts", "IMPLEMENTATION_SUMMARY.md"):
            with self.subTest(path=path):
                self.assertEqual(rules({path: Change(added=["x"], new=True)}), ["junk-files"])
        self.assertEqual(rules({"src/parser_v2.py": Change(added=["x = 1"])}), [])

    def test_does_not_treat_ordinary_names_as_junk(self):
        for path in ("src/renew.py", "src/base64.py", "docs/README.md", "src/news.ts"):
            with self.subTest(path=path):
                self.assertEqual(rules({path: Change(added=["x"], new=True)}), [])

    def test_size_counts_code_but_not_tests_docs_or_blank_lines(self):
        many = ["x = 1"] * (MAX_CODE_LINES + 1)
        self.assertEqual(rules({"src/a.py": Change(added=many)}), ["size"])
        self.assertEqual(rules({"tests/test_a.py": Change(added=many)}), [])
        self.assertEqual(rules({"README.md": Change(added=many)}), [])
        self.assertEqual(rules({"src/a.py": Change(added=[*many[:-1], "", "  "])}), [])

    def test_flags_go_and_kotlin_suppressions(self):
        for path, line in (
            ("internal/agent/loop.go", "x := f() //nolint:errcheck"),
            ("android/app/src/main/java/a/B.kt", '@Suppress("UNCHECKED_CAST")'),
        ):
            with self.subTest(path=path):
                self.assertEqual(rules({path: Change(added=[line])}), ["suppressions"])

    def test_flags_skipped_go_tests(self):
        change = Change(added=['	t.Skip("flaky")'])
        self.assertEqual(rules({"internal/agent/loop_test.go": change}), ["skipped-tests"])

    def test_counts_go_test_functions(self):
        change = Change(removed=["func TestA(t *testing.T) {", "func TestB(t *testing.T) {"])
        self.assertEqual(rules({"internal/agent/loop_test.go": change}), ["tests-removed"])

    def test_go_tests_and_fixtures_do_not_count_towards_size(self):
        many = ["x := 1"] * (MAX_CODE_LINES + 1)
        for path in ("internal/a/a_test.go", "bench/fixtures/t4/server.go", "x/testdata/a.go"):
            with self.subTest(path=path):
                self.assertEqual(rules({path: Change(added=many)}), [])
        self.assertEqual(rules({"internal/a/a.go": Change(added=many)}), ["size"])

    def test_flags_catalan_copy_names(self):
        for path in ("internal/agent/loop_nou.go", "internal/web/app_antic.js"):
            with self.subTest(path=path):
                self.assertEqual(rules({path: Change(added=["x"], new=True)}), ["junk-files"])

    def test_warns_when_gates_or_manifests_change(self):
        for path in (".golangci.yml", ".claude/hooks/guard.py", "go.mod", "desktop/package.json"):
            with self.subTest(path=path):
                self.assertEqual(rules({path: Change(added=["x"])}), ["gates"])


if __name__ == "__main__":
    unittest.main()
