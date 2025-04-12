# watcher.py
import subprocess
import os

os.chdir(os.path.dirname(os.path.abspath(__file__)))

while True:
    process = subprocess.Popen(["python3", "main.py"])
    process.wait()