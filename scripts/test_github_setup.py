"""Exercise administration boundaries offline; never contact GitHub."""
import importlib.util
import contextlib
import io
from pathlib import Path
import subprocess
import sys
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('setup_github', Path(__file__).with_name('setup-github.py'))
setup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(setup)


class GitHubSetupTest(unittest.TestCase):
    def setUp(self):
        self.commands = []
        self.requests = []
        self.dirty = False
        self.origin = ''
        self.private = False
        self.missing = False

    def command(self, *args, **kwargs):
        self.commands.append(args)
        return {
            ('git', 'branch', '--show-current'): 'main\n',
            ('git', 'rev-parse', '--verify', 'HEAD'): 'synthetic-sha\n',
            ('git', 'status', '--porcelain'): ' M README.md\n' if self.dirty else '',
            ('git', 'remote'): 'origin\n' if self.origin else '',
            ('git', 'remote', 'get-url', 'origin'): self.origin,
        }.get(args, '')

    def api(self, path, method='GET', payload=None, missing_ok=False):
        self.requests.append((path, method, payload))
        if method == 'GET' and path == 'repos/' + setup.REPO:
            if self.missing:
                self.missing = False
                return None
            return {'full_name': setup.REPO, 'private': self.private, 'archived': False}
        return None if missing_ok else {}

    def apply(self):
        with patch.object(sys, 'argv', ['setup-github.py', '--apply']), patch.object(setup, 'run', self.command), patch.object(setup, 'api', self.api):
            with contextlib.redirect_stdout(io.StringIO()): setup.main()

    def test_dry_run_has_no_git_or_remote_access(self):
        with patch.object(sys, 'argv', ['setup-github.py', '--dry-run']), patch.object(setup, 'run') as run, patch.object(setup, 'api') as api:
            with contextlib.redirect_stdout(io.StringIO()): setup.main()
            run.assert_not_called()
            api.assert_not_called()

    def test_dirty_checkout_stops_before_remote_mutation(self):
        self.dirty = True
        with self.assertRaises(ValueError): self.apply()
        self.assertEqual(self.requests, [])
        self.assertFalse(any('push' in call for call in self.commands))

    def test_foreign_origin_is_never_replaced_or_pushed(self):
        self.origin = 'https://github.com/example/other.git'
        with self.assertRaises(ValueError): self.apply()
        self.assertEqual(self.requests, [])
        self.assertFalse(any('push' in call or 'set-url' in call for call in self.commands))

    def test_existing_private_repo_is_not_silently_made_public(self):
        self.private = True
        with self.assertRaises(ValueError): self.apply()
        self.assertFalse(any(method != 'GET' for _, method, _ in self.requests))
        self.assertFalse(any('push' in call for call in self.commands))

    def test_bootstrap_is_limited_to_target_repo_and_never_releases(self):
        self.missing = True
        self.apply()
        self.assertIn(('git', 'push', '--set-upstream', 'origin', 'main'), self.commands)
        self.assertTrue(all(path.startswith('repos/' + setup.REPO) for path, _, _ in self.requests))
        self.assertFalse(any('tag' in call or '--force' in call or 'npm' in call for call in self.commands))
        protection = next(payload for path, method, payload in self.requests if path.endswith('/protection'))
        self.assertTrue(protection['enforce_admins'])
        self.assertFalse(protection['allow_force_pushes'])
        self.assertEqual(protection['required_status_checks']['contexts'], ['test (ubuntu-latest)', 'test (macos-latest)', 'isolation'])
        self.assertIn(('gh', 'workflow', 'run', 'pages.yml', '--repo', setup.REPO, '--ref', 'main'), self.commands)

    def test_denied_remote_read_is_not_treated_as_missing(self):
        error = subprocess.CalledProcessError(1, ['gh'], stderr='Resource not accessible by integration (HTTP 403)')
        with patch.object(setup, 'run', side_effect=error):
            with self.assertRaises(subprocess.CalledProcessError):
                setup.api('repos/' + setup.REPO, missing_ok=True)


if __name__ == '__main__':
    unittest.main()
