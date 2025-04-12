import subprocess
import os
import sys
import tempfile
import shutil

def extract_main_path():
    if hasattr(sys, "_MEIPASS"):
        # We are in a PyInstaller bundle
        source_path = os.path.join(sys._MEIPASS, "bot", "main.py")
        tmpdir = tempfile.mkdtemp()
        extracted_main = os.path.join(tmpdir, "main.py")
        shutil.copyfile(source_path, extracted_main)
        return extracted_main, tmpdir
    else:
        # We are running in a dev environment
        return os.path.join(os.path.dirname(__file__), "main.py"), None

while True:
    main_path, tmpdir = extract_main_path()
    try:
        process = subprocess.Popen(["python3", main_path])
        process.wait()
    except Exception as e:
        print(f"Failed to run main.py: {e}")
    finally:
        if tmpdir:
            shutil.rmtree(tmpdir, ignore_errors=True)
        print("main.py exited, restarting...")