# watcher.py
import subprocess
import os
import sys
import tempfile
import shutil

def extract_main_script():
    if hasattr(sys, "_MEIPASS"):
        # We are inside a PyInstaller onefile bundle
        bundled_path = os.path.join(sys._MEIPASS, "bot", "main.py")
    else:
        # Dev environment
        bundled_path = os.path.join(os.path.dirname(__file__), "main.py")
    return bundled_path

def run_main():
    main_path = extract_main_script()

    if hasattr(sys, "_MEIPASS"):
        # Copy to temp dir so subprocess can read it
        tmpdir = tempfile.mkdtemp()
        tmp_main = os.path.join(tmpdir, "main.py")
        shutil.copyfile(main_path, tmp_main)
        cmd = ["python3", tmp_main]
    else:
        cmd = ["python3", main_path]

    return subprocess.Popen(cmd)

while True:
    process = run_main()
    process.wait()