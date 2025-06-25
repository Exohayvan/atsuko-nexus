import sys
import os
import time
import requests
import shutil
import platform
import subprocess
import zipfile

# === CONFIG ===
REPO = "Exohayvan/atsuko-nexus"
script_dir = os.path.dirname(os.path.abspath(__file__))
MAIN_NAME = "main.exe" if platform.system().lower() == "windows" else "main"

def get_system_key():
    os_type = platform.system().lower()
    if os_type == "windows":
        return "windows-x64"
    elif os_type == "darwin":
        return "macos-x64"
    elif os_type == "linux":
        return "linux-x64"
    else:
        raise RuntimeError(f"Unsupported platform: {os_type}")

def get_download_url():
    system_key = get_system_key()
    api_url = f"https://api.github.com/repos/{REPO}/releases/latest"

    print(f"[Updater] System key: {system_key}")
    response = requests.get(api_url, timeout=10)
    response.raise_for_status()

    for asset in response.json().get("assets", []):
        name = asset.get("name", "").lower()
        if system_key in name:
            print(f"[Updater] Matched asset: {name}")
            return asset["browser_download_url"], name

    raise RuntimeError(f"No matching asset found for system key: {system_key}")

def wait_for_close(target_file, timeout=10):
    for _ in range(timeout):
        try:
            os.rename(target_file, target_file)
            return True
        except:
            time.sleep(1)
    return False

def download_file(url, dest_path):
    response = requests.get(url, stream=True)
    response.raise_for_status()
    with open(dest_path, "wb") as f:
        shutil.copyfileobj(response.raw, f)

def extract_and_replace(zip_path, extract_to_dir):
    with zipfile.ZipFile(zip_path, "r") as zip_ref:
        zip_ref.extractall(extract_to_dir)
        extracted_names = zip_ref.namelist()

    # Find the correct 'main' file
    extracted_file = next(
        (f for f in extracted_names if os.path.basename(f) == MAIN_NAME and not f.endswith("/")),
        None
    )

    if not extracted_file:
        raise RuntimeError(f"Could not find '{MAIN_NAME}' in the zip.")

    extracted_path = os.path.join(extract_to_dir, extracted_file)
    target_path = os.path.join(script_dir, MAIN_NAME)

    print("[Updater] Replacing old binary...")

    if os.path.exists(target_path):
        os.remove(target_path)
        print("[Updater] Old binary removed.")

    shutil.move(extracted_path, target_path)
    os.chmod(target_path, 0o755)
    print(f"[Updater] New binary placed: {target_path}")
    return target_path

def relaunch(path):
    if platform.system().lower() == "windows":
        os.startfile(path)
    else:
        subprocess.Popen([path])

if __name__ == "__main__":
    try:
        print("[Updater] Starting updater...")
        download_url, zip_filename = get_download_url()
        zip_path = os.path.join(script_dir, zip_filename)

        print(f"[Updater] Downloading zip: {zip_filename}")
        download_file(download_url, zip_path)

        print("[Updater] Extracting and replacing...")
        new_path = extract_and_replace(zip_path, script_dir)

        print("[Updater] Cleaning up...")
        os.remove(zip_path)

        print("[Updater] Relaunching updated binary...")
        relaunch(new_path)

    except Exception as e:
        print(f"[Updater] Error: {e}")
