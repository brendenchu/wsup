import json
import re
import sys
import unittest

from migration_baseline import DATA, SOURCE, compare_diagnostics, replay, test_map
import test_migration_safety


class MigrationBaselineTest(unittest.TestCase):
    def test_diagnostic_failure_contracts(self):
        command = [sys.executable, "-B", str(SOURCE / "src/wsup.py")]
        self.assertEqual(20, compare_diagnostics(command, command))

    def test_go_coverage_references_original_contracts_and_existing_tests(self):
        progress = json.loads((DATA / "go-coverage.json").read_text())
        self.assertEqual(1, progress["version"])
        self.assertEqual("3b709d522ddd1aab38c4442b20cfb22eb1787915",
                         progress["baseline_commit"])
        self.assertEqual("pending", progress["unlisted_status"])
        originals = {entry["python"] for entry in test_map(SOURCE)}
        seen = set()
        for entry in progress["entries"]:
            with self.subTest(contract=entry["python"]):
                self.assertIn(entry["python"], originals)
                self.assertNotIn(entry["python"], seen)
                seen.add(entry["python"])
                self.assertIn(entry["status"], ("covered", "partial"))
                self.assertTrue(entry["assertions"])
                self.assertEqual(entry["status"] == "partial", bool(entry["remaining"]))
                self.assertTrue(entry["tests"])
                for filename, names in entry["tests"].items():
                    self.assertTrue(filename.startswith("internal/"))
                    self.assertTrue(filename.endswith("_test.go"))
                    path = (SOURCE / filename).resolve()
                    self.assertTrue(path.is_relative_to(SOURCE.resolve()))
                    body = path.read_text()
                    self.assertTrue(names)
                    self.assertEqual(len(names), len(set(names)))
                    for name in names:
                        self.assertRegex(body, r"(?m)^func " + re.escape(name) +
                                         r"\(t \*testing\.T\) \{")

    def test_python_command_contracts(self):
        expected = json.loads((DATA / "expected.json").read_text())
        actual = replay([sys.executable, "-B", str(SOURCE / "src/wsup.py")])
        for name in expected:
            with self.subTest(scenario=name):
                self.assertEqual(expected[name], actual[name])

    def test_all_original_tests_have_pending_go_targets(self):
        recorded = json.loads((DATA / "test-map.json").read_text())
        actual = test_map(SOURCE)
        self.assertEqual(recorded, actual)

    def test_safety_contracts_reference_real_tests_and_assertions(self):
        specs = json.loads((DATA / "safety-contracts.json").read_text())
        self.assertEqual(1, specs["version"])
        self.assertEqual("3b709d522ddd1aab38c4442b20cfb22eb1787915",
                         specs["baseline_commit"])
        originals = {entry["python"].split(".")[-1] for entry in test_map(SOURCE)}
        ids = set()
        for contract in specs["contracts"]:
            self.assertNotIn(contract["id"], ids)
            ids.add(contract["id"])
            self.assertTrue(contract["setup"])
            self.assertTrue(contract["stimulus"])
            self.assertTrue(contract["assertions"])
            for name in contract["python_tests"]:
                self.assertIn(name, originals, contract["id"])
            for name in contract.get("additional_tests", []):
                self.assertTrue(callable(getattr(
                    test_migration_safety.MigrationSafetyTest, name)), contract["id"])
