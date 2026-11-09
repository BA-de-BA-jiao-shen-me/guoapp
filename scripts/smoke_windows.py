import argparse
import json
import platform
import re
import subprocess
import tempfile
import zipfile
from pathlib import Path

from app_build import BuildVariant, add_variant_argument

root = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser()
    add_variant_argument(parser)
    variant = BuildVariant(parser.parse_args().all_sources)
    if platform.system() != 'Windows':
        raise SystemExit('此检查需要 Windows。')
    version = re.search(r'^version:\s*(\S+)', (root / 'pubspec.yaml').read_text(encoding='utf-8'), re.MULTILINE).group(1)
    package = root / 'dist' / 'windows' / f'{variant.slug}-{version}-windows-x64.zip'
    with tempfile.TemporaryDirectory(prefix='zhenguojian-smoke-') as temporary:
        directory = Path(temporary)
        with zipfile.ZipFile(package) as archive:
            archive.extractall(directory)
        media = directory / 'fixture.mp4'
        subprocess.run(['ffmpeg', '-v', 'error', '-y', '-f', 'lavfi',
                        '-i', 'testsrc2=size=160x90:rate=12', '-t', '3',
                        '-c:v', 'libx264', '-threads', '1', str(media)], check=True)
        report = directory / 'result.json'
        completed = subprocess.run(
            [str(directory / (variant.slug + '.exe')), '--package-smoke', str(report), str(media)],
            cwd=directory,
            capture_output=True,
            text=True,
            timeout=90,
        )
        if completed.stdout:
            print(completed.stdout, end='' if completed.stdout.endswith('\n') else '\n')
        if completed.stderr:
            print(completed.stderr, end='' if completed.stderr.endswith('\n') else '\n')
        evidence_text = report.read_text(encoding='utf-8') if report.exists() else ''
        if evidence_text:
            print(evidence_text if evidence_text.endswith('\n') else evidence_text + '\n', end='')
        if completed.returncode != 0:
            detail = evidence_text or f'exit={completed.returncode}'
            raise SystemExit(f'Windows 包启动验收失败: {detail}')
        evidence = json.loads(evidence_text)
        if evidence.get('ok') is not True:
            raise SystemExit(f'Windows 包启动验收未通过: {evidence}')
        output = root / 'build' / 'windows-package-smoke.json'
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(json.dumps(evidence, indent=2) + '\n')
        print(json.dumps(evidence))


if __name__ == '__main__':
    main()
