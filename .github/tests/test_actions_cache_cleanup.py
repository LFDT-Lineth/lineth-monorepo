"""Run with python3 .github/tests/test_actions_cache_cleanup.py (requires bash and jq)."""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import textwrap
import unittest


WORKFLOW = Path(__file__).resolve().parents[1] / "workflows/actions-cache-cleanup.yml"
BUDGET = 2 * 1024**3


def cache(cache_id, size, ref="refs/pull/1/head", key="codeql-dependencies-1-Linux-go-a", day=1):
  return {
    "id": cache_id,
    "size_in_bytes": size,
    "ref": ref,
    "key": key,
    "last_accessed_at": f"2026-09-{day:02d}T00:00:00.000000Z",
    "created_at": "2026-09-01T00:00:00.000000Z",
  }


class CacheBudgetTest(unittest.TestCase):
  @classmethod
  def setUpClass(cls):
    # Exercise the actual named run block without adding a YAML parser dependency.
    step = WORKFLOW.read_text().split("      - name: Limit PR CodeQL dependency caches to 2 GiB\n")[1]
    block = re.search(r"        run: \|\n((?:          .*\n|\n)+)", step)
    cls.script = textwrap.dedent(block.group(1))
    cls.budget = re.search(r'CODEQL_PR_CACHE_BUDGET_BYTES: "(\d+)"', step).group(1)

  def run_cleanup(self, pages, list_failure=False, delete_failure=False):
    with tempfile.TemporaryDirectory() as directory:
      root = Path(directory)
      (root / "inventory").write_text("\n".join(json.dumps({"actions_caches": p}) for p in pages))
      (root / "gh").write_text(textwrap.dedent("""\
        #!/usr/bin/env python3
        import os
        from pathlib import Path
        import sys

        root = Path(os.environ["FIXTURE_DIR"])
        if sys.argv[1:] == ["api", "--paginate", "repos/test/repo/actions/caches?per_page=100"]:
          print((root / "inventory").read_text())
          sys.exit(int(os.environ["LIST_FAILURE"]))
        assert sys.argv[1:4] == ["api", "-X", "DELETE"], sys.argv
        assert sys.argv[4].startswith("repos/test/repo/actions/caches/"), sys.argv
        with (root / "deletions").open("a") as output:
          output.write(sys.argv[4].rsplit("/", 1)[1] + "\\n")
        sys.exit(int(os.environ["DELETE_FAILURE"]))
        """))
      (root / "gh").chmod(0o755)
      result = subprocess.run(
        ["bash", "-c", self.script], capture_output=True, text=True,
        env={
          **os.environ,
          "PATH": directory + os.pathsep + os.environ["PATH"],
          "REPO": "test/repo", "CODEQL_PR_CACHE_BUDGET_BYTES": self.budget,
          "FIXTURE_DIR": directory, "LIST_FAILURE": str(int(list_failure)),
          "DELETE_FAILURE": str(int(delete_failure)),
        },
      )
      deleted = root / "deletions"
      ids = [int(value) for value in deleted.read_text().splitlines()] if deleted.exists() else []
      return result, ids

  def assert_cleanup(self, pages, expected):
    result, ids = self.run_cleanup(pages)
    self.assertEqual(result.returncode, 0, result.stderr)
    self.assertEqual(ids, expected)

  def test_empty_below_and_exact_budget(self):
    self.assertEqual(int(self.budget), BUDGET)
    for entries in ([], [cache(1, 1)], [cache(1, BUDGET)]):
      with self.subTest(entries=entries):
        self.assert_cleanup([entries], [])

  def test_global_budget_across_pages_and_prs(self):
    self.assert_cleanup([
      [cache(1, BUDGET // 2, day=1), cache(3, BUDGET // 2, ref="refs/pull/3/merge", day=3)],
      [cache(2, BUDGET // 2, ref="refs/pull/2/head", day=2)],
    ], [1])

  def test_preserves_other_refs_and_cache_families(self):
    excluded = [cache(i, BUDGET * 2, ref=ref) for i, ref in enumerate([
      "refs/heads/main", "refs/heads/feature", "refs/tags/v1", "refs/pull/1/head/extra",
      "refs/pull/not-a-number/head", "other/refs/pull/1/head",
    ], 10)]
    excluded += [cache(20 + i, BUDGET * 2, key=key) for i, key in enumerate([
      "gradle-dependencies-v1-a", "node-cache-pnpm-a", "setup-go-a", "setup-zig-a",
      "other-codeql-dependencies-a", "codeql-trap-a",
    ])]
    self.assert_cleanup([excluded + [cache(1, BUDGET)]], [])

  def test_older_key_on_same_pr_is_removed_under_pressure(self):
    self.assert_cleanup([[
      cache(1, BUDGET, key="codeql-dependencies-1-Linux-go-old"),
      cache(2, BUDGET, key="codeql-dependencies-1-Linux-go-new", day=2),
    ]], [1])

  def test_last_access_takes_precedence_over_creation(self):
    old_but_used = cache(1, BUDGET, day=3)
    new_but_unused = cache(2, BUDGET, day=2)
    new_but_unused["created_at"] = "2026-09-02T00:00:00.000000Z"
    self.assert_cleanup([[new_but_unused, old_but_used]], [2])

  def test_equal_timestamps_use_id_for_deterministic_order(self):
    self.assert_cleanup([[cache(1, BUDGET), cache(2, BUDGET)]], [1])

  def test_oversized_newest_entry_leaves_no_retained_prefix(self):
    self.assert_cleanup([[cache(1, 1), cache(2, BUDGET + 1, day=2)]], [2, 1])

  def test_failed_inventory_never_deletes_even_with_partial_output(self):
    result, ids = self.run_cleanup([[cache(1, BUDGET + 1)]], list_failure=True)
    self.assertNotEqual(result.returncode, 0)
    self.assertEqual(ids, [])

  def test_failed_delete_fails_step_and_stops(self):
    result, ids = self.run_cleanup(
      [[cache(1, BUDGET), cache(2, BUDGET), cache(3, BUDGET)]], delete_failure=True,
    )
    self.assertNotEqual(result.returncode, 0)
    self.assertEqual(ids, [2])


if __name__ == "__main__":
  unittest.main()
