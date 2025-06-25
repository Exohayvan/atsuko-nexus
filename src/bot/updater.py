import sys
import os
import time
import requests
import shutil

def wait_for_close(target_file, timeout=10):
    for _ in range(timeout):
        try:
            os.rename(target_file, target_file)
            return True
        except:
            time.sleep(1)
    return False

def download_exe(url, destination):
    response = requests.get(url, stream=True)
    temp_path = destination + ".new"
    with open(temp_path, "wb") as f:
        shutil.copyfileobj(response.raw, f)
    os.replace(temp_path, destination)

def relaunch(path):
    try:
        os.startfile(path)
    except AttributeError:
        import subprocess
        subprocess.Popen([path])

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: updater.exe <path_to_main_exe> <download_url>")
        sys.exit(1)

    main_exe = sys.argv[1]
    download_url = sys.argv[2]

    print("[Updater] Waiting for main to exit...")
    if wait_for_close(main_exe):
        try:
            print("[Updater] Downloading update...")
            download_exe(download_url, main_exe)
            print("[Updater] Relaunching main...")
            relaunch(main_exe)
        except Exception as e:
            print(f"[Updater] Failed to update: {e}")
    else:
        print("[Updater] Could not acquire lock. Update aborted.")