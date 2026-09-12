"""Run CI's test-only native GUI and check its report; no browser automation library."""
from pathlib import Path
import os
import platform
import subprocess
import tempfile

system = {'Darwin': 'darwin', 'Windows': 'windows', 'Linux': 'linux'}[platform.system()]
arch = 'arm64' if platform.machine().lower() in ('arm64', 'aarch64') else 'amd64'
subprocess.run(['go', 'run', './scripts/build-gui.go', '-smoke', '-target', f'{system}-{arch}'], check=True)
base = Path('dist') / f'gui-{system}-{arch}'
exe = base / ('browser-session-gui.exe' if system == 'windows' else 'browser-session-gui')
if system == 'darwin':
    exe = base / 'Browser Sessions.app/Contents/MacOS/browser-session-gui'
command = [str(exe.resolve())]
if system == 'linux':
    command = ['xvfb-run', '-a', *command]
with tempfile.TemporaryDirectory(prefix='gui-report-') as tmp:
    report = Path(tmp) / 'result.txt'
    env = os.environ.copy()
    env['BROWSER_SESSION_GUI_REPORT'] = str(report)
    subprocess.run(command, env=env, check=True, timeout=90)
    result = report.read_text() if report.exists() else 'FAIL: native UI did not report'
    print(result)
    if not result.startswith('PASS:'):
        raise SystemExit(1)
