import subprocess
import os
import sys
import tempfile
import shutil

def extract_main_path():
    if hasattr(sys, "_MEIPASS"):
        # Running in a PyInstaller bundle
        source_path = os.path.join(sys._MEIPASS, "bot", "main.py")
        tmpdir = tempfile.mkdtemp()
        extracted_main = os.path.join(tmpdir, "main.py")
        shutil.copyfile(source_path, extracted_main)
        return extracted_main, tmpdir
    else:
        # Dev mode
        return os.path.join(os.path.dirname(__file__), "main.py"), None

def get_python_executable():
    if hasattr(sys, "_MEIPASS"):
        # PyInstaller mode — use system Python, not self
        return shutil.which("python3") or shutil.which("python")
    else:
        # Dev mode — use current Python
        return sys.executable

while True:
    main_path, tmpdir = extract_main_path()
    try:
        python_exec = get_python_executable()
        if not python_exec:
            raise RuntimeError("No system Python interpreter found.")
        process = subprocess.Popen([python_exec, main_path])
        process.wait()
    except Exception as e:
        print(f"Failed to run main.py: {e}")
    finally:
        if tmpdir:
            shutil.rmtree(tmpdir, ignore_errors=True)
        print("main.py exited, restarting...")