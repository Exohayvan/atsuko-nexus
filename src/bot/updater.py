import sys
import os
import time
import requests
import shutil
import platform
import subprocess
import zipfile

REPO = "Exohayvan/atsuko-nexus"
MAIN_NAME = "atsuko-nexus.exe" if platform.system().lower() == "windows" else "atsuko-nexus"

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

    new_path = os.path.join(extract_to_dir, MAIN_NAME)
    old_path = os.path.join(os.getcwd(), MAIN_NAME)

    if os.path.exists(old_path):
        os.remove(old_path)
        print("[Updater] Old binary removed.")

    shutil.move(new_path, old_path)
    print("[Updater] New binary placed.")

    os.chmod(old_path, 0o755)
    return old_path

def relaunch(path):
    if platform.system().lower() == "windows":
        os.startfile(path)
    else:
        subprocess.Popen([path])

if __name__ == "__main__":
    try:
        print("[Updater] Starting updater...")
        download_url, zip_filename = get_download_url()
        zip_path = os.path.join(os.getcwd(), zip_filename)

        print(f"[Updater] Downloading zip: {zip_filename}")
        download_file(download_url, zip_path)

        print("[Updater] Extracting and replacing old binary...")
        new_exe_path = extract_and_replace(zip_path, os.getcwd())

        print("[Updater] Cleaning up...")
        os.remove(zip_path)

        print("[Updater] Relaunching new version...")
        relaunch(new_exe_path)

    except Exception as e:
        print(f"[Updater] Error: {e}")
