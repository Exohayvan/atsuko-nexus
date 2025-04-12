import subprocess
import os
import sys
import tempfile
import shutil

def extract_main_script():
    if hasattr(sys, "_MEIPASS"):
        # Inside PyInstaller bundle
        return os.path.join(sys._MEIPASS, "bot", "main.py")
    else:
        # Running in dev environment
        return os.path.join(os.path.dirname(__file__), "main.py")

def run_main():
    main_path = extract_main_script()

    if hasattr(sys, "_MEIPASS"):
        # Copy bundled main.py to a temporary file so subprocess can run it
        tmpdir = tempfile.mkdtemp()
        tmp_main = os.path.join(tmpdir, "main.py")
        shutil.copyfile(main_path, tmp_main)
        cmd = [sys.executable, tmp_main]
    else:
        cmd = [sys.executable, main_path]

    return subprocess.Popen(cmd)

while True:
    process = run_main()
    process.wait()