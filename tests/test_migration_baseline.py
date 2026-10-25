import json
import sys
import unittest

from migration_baseline import DATA, SOURCE, replay, test_map


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
