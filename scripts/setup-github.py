#!/usr/bin/env python3
"""Configure only the authorized Chat CLI repository; never publish a version."""
import argparse
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
REPO = 'AngelMsger/wecom-chat-cli'
URL = 'https://github.com/' + REPO + '.git'
SITE = 'https://angelmsger.github.io/wecom-chat-cli/'
DESCRIPTION = 'WeCom smart-bot conversations for agents, with scoped tasks and independently reviewed replies.'


def run(*args, payload=None, capture=True):
    return subprocess.run(args, cwd=ROOT, input=json.dumps(payload) if payload is not None else None,
                          text=True, capture_output=capture, check=True).stdout


def api(path, method='GET', payload=None, missing_ok=False):
    args = ['gh', 'api', path, '--method', method]
    if payload is not None:
        args += ['--input', '-']
    try:
        output = run(*args, payload=payload)
    except subprocess.CalledProcessError as error:
        if missing_ok and '(HTTP 404)' in (error.stderr or ''):
            return None
        raise
    return json.loads(output) if output.strip() else {}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--apply', action='store_true')
    group.add_argument('--dry-run', action='store_true')
    args = parser.parse_args()
    if args.dry_run:
        print(json.dumps({'repository': REPO, 'visibility': 'public', 'site': SITE,
                          'steps': ['create if absent', 'push clean main without force',
                                    'enable Pages via Actions', 'set metadata and security reporting',
                                    'protect main', 'dispatch Pages'], 'release': False}, indent=2))
        return
    if run('git', 'branch', '--show-current').strip() != 'main':
        raise ValueError('Run from the reviewed main branch.')
    run('git', 'rev-parse', '--verify', 'HEAD')
    if run('git', 'status', '--porcelain').strip():
        raise ValueError('Commit the reviewed files before repository bootstrap.')
    remotes = run('git', 'remote').splitlines()
    if 'origin' in remotes and run('git', 'remote', 'get-url', 'origin').strip() != URL:
        raise ValueError('Existing origin differs; inspect it instead of replacing it.')
    repo = api('repos/' + REPO, missing_ok=True)
    if repo is None:
        run('gh', 'repo', 'create', REPO, '--public', '--description', DESCRIPTION, '--homepage', SITE)
        repo = api('repos/' + REPO)
    if repo['full_name'] != REPO or repo['private'] or repo['archived']:
        raise ValueError('Repository identity, visibility or archived status differs from the plan.')
    if 'origin' not in remotes:
        run('git', 'remote', 'add', 'origin', URL)
    run('git', 'push', '--set-upstream', 'origin', 'main', capture=False)
    api('repos/' + REPO, 'PATCH', {'description': DESCRIPTION, 'homepage': SITE,
                                 'default_branch': 'main', 'has_issues': True, 'has_wiki': False})
    api('repos/' + REPO + '/topics', 'PUT', {'names': ['wecom', 'smart-bot', 'cli', 'ai-agents', 'golang', 'security']})
    if api('repos/' + REPO + '/pages', missing_ok=True) is None:
        api('repos/' + REPO + '/pages', 'POST', {'build_type': 'workflow'})
    else:
        api('repos/' + REPO + '/pages', 'PUT', {'build_type': 'workflow'})
    api('repos/' + REPO + '/private-vulnerability-reporting', 'PUT')
    api('repos/' + REPO + '/actions/permissions/workflow', 'PUT',
        {'default_workflow_permissions': 'read', 'can_approve_pull_request_reviews': False})
    api('repos/' + REPO + '/branches/main/protection', 'PUT', {
        'required_status_checks': {'strict': True, 'contexts': ['test (ubuntu-latest)', 'test (macos-latest)', 'isolation']},
        'enforce_admins': True, 'required_pull_request_reviews': {'dismiss_stale_reviews': True, 'required_approving_review_count': 0},
        'restrictions': None, 'required_conversation_resolution': True,
        'allow_force_pushes': False, 'allow_deletions': False})
    run('gh', 'workflow', 'run', 'pages.yml', '--repo', REPO, '--ref', 'main')
    print(json.dumps({'repository': 'https://github.com/' + REPO, 'site': SITE,
                      'status': 'configured; verify CI and Pages runs before declaring deployment complete'}, indent=2))


if __name__ == '__main__':
    try:
        main()
    except subprocess.CalledProcessError as error:
        print(error.stderr or 'GitHub setup command failed.', file=sys.stderr)
        sys.exit(1)
    except ValueError as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
