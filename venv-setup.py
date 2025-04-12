import os
import subprocess
import sys
import venv
from pathlib import Path

# Set the path for the virtual environment
venv_dir = Path('./.venv')
requirements_path = Path('./requirements.txt')

# Step 1: Create the virtual environment if it doesn't exist
if not venv_dir.exists():
    print(f"Creating virtual environment at {venv_dir}...")
    venv.create(venv_dir, with_pip=True)
else:
    print(f"Virtual environment already exists at {venv_dir}")

# Step 2: Determine the path to the pip executable inside the venv
pip_path = venv_dir / ('Scripts/pip.exe' if os.name == 'nt' else 'bin/pip')

# Step 3: Install the requirements
if requirements_path.exists():
    print(f"Installing requirements from {requirements_path}...")
    subprocess.check_call([str(pip_path), 'install', '-r', str(requirements_path)])
    print("Requirements installed successfully.")
else:
    print(f"requirements.txt not found at {requirements_path}")
    sys.exit(1)