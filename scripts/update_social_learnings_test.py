#!/usr/bin/env python3
"""Unit tests for social learning sync."""

import json
import os
import tempfile
import unittest
from pathlib import Path

# Import the module under test
import sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from update_social_learnings import add_social_learning, load_social_learnings, synthesize_social


class TestAddSocialLearning(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.original_cwd = os.getcwd()
        os.chdir(self.temp_dir)

    def tearDown(self):
        os.chdir(self.original_cwd)

    def test_add_creates_file_if_missing(self):
        """Should create social_learnings.jsonl if it doesn't exist."""
        add_social_learning("Test insight", "discussion")
        self.assertTrue(os.path.exists("memory/social_learnings.jsonl"))

    def test_add_appends_to_existing(self):
        """Should append to existing file."""
        add_social_learning("First insight", "discussion")
        add_social_learning("Second insight", "issue")
        
        learnings = load_social_learnings()
        self.assertEqual(len(learnings), 2)

    def test_add_includes_metadata(self):
        """Should include timestamp and source."""
        add_social_learning("Test insight", "discussion")
        
        learnings = load_social_learnings()
        self.assertEqual(learnings[0]["type"], "social")
        self.assertEqual(learnings[0]["source"], "discussion")
        self.assertEqual(learnings[0]["insight"], "Test insight")
        self.assertIn("timestamp", learnings[0])


class TestLoadSocialLearnings(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.original_cwd = os.getcwd()
        os.chdir(self.temp_dir)

    def tearDown(self):
        os.chdir(self.original_cwd)

    def test_load_returns_empty_for_missing_file(self):
        """Should return empty list if file doesn't exist."""
        learnings = load_social_learnings()
        self.assertEqual(learnings, [])

    def test_load_handles_corrupt_lines(self):
        """Should skip corrupt JSON lines and load valid ones."""
        os.makedirs("memory", exist_ok=True)
        with open("memory/social_learnings.jsonl", "w") as f:
            f.write('{"type": "social", "insight": "valid 1", "source": "test"}\n')
            f.write('corrupt line\n')
            f.write('{"type": "social", "insight": "valid 2", "source": "test"}\n')
        
        learnings = load_social_learnings()
        # Should load the 2 valid lines, skip the corrupt one
        self.assertEqual(len(learnings), 2)


class TestSynthesizeSocial(unittest.TestCase):
    def test_empty_learnings(self):
        """Should return placeholder for empty learnings."""
        result = synthesize_social([])
        self.assertIn("No social interactions", result)

    def test_groups_by_source(self):
        """Should group insights by source."""
        learnings = [
            {"source": "discussion", "insight": "Disc insight 1"},
            {"source": "issue", "insight": "Issue insight 1"},
            {"source": "discussion", "insight": "Disc insight 2"},
        ]
        result = synthesize_social(learnings)
        self.assertIn("From Discussion", result)
        self.assertIn("From Issue", result)

    def test_limits_to_recent_five(self):
        """Should show only latest 5 insights per source."""
        learnings = [
            {"source": "discussion", "insight": f"Insight {i}"}
            for i in range(10)
        ]
        result = synthesize_social(learnings)
        # Count dashes - should have 5, not 10
        self.assertEqual(result.count("- Insight"), 5)


if __name__ == "__main__":
    unittest.main()
