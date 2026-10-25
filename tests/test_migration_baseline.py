import json
import sys
import unittest

from migration_baseline import DATA, SOURCE, replay, test_map
import test_migration_safety


class MigrationBaselineTest(unittest.TestCase):
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
