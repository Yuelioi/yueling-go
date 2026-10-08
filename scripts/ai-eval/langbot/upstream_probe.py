"""Run selected upstream component tests without application startup imports."""
import os
import sys
import types
from pathlib import Path
import pytest

source = Path(os.environ['LANGBOT_SOURCE']).resolve()
output = Path(os.environ['AI_EVAL_OUTPUT']).resolve()
output.mkdir(parents=True, exist_ok=True)
# Application is only a type dependency of tested components. Whole application
# startup, external platforms and database persistence are outside this probe.
sys.modules['langbot.pkg.core.app'] = types.ModuleType('langbot.pkg.core.app')
tests = [
    'tests/unit_tests/platform/test_http_bot_tenancy.py',
    'tests/unit_tests/provider/test_session_manager.py',
    'tests/unit_tests/provider/test_localagent_tool_content.py',
    'tests/unit_tests/provider/test_localagent_no_duplicate.py',
]
raise SystemExit(pytest.main([
    *[str(source / name) for name in tests], '-q',
    f'--junitxml={output / "upstream-results.xml"}',
    '-o', f'cache_dir={output / "pytest-cache"}',
]))
