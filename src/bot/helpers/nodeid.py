import hashlib
import os
import platform
import subprocess
import logging
from helpers.clogging import setup_logging

setup_logging()
logger = logging.getLogger("NODEID")

class NodeIDGenerator:
    def __init__(self):
        self.system = platform.system()

    def get_node_id(self):
        logger.debug("Collecting Node Fingerprint Parts...")
        parts = self._get_fingerprint_parts()
        fingerprint = "|".join(filter(None, parts))  # Remove empty strings
        return hashlib.sha256(fingerprint.encode()).hexdigest()

    def _get_fingerprint_parts(self):
        try:
            if self.system == "Linux":
                logger.debug("Linux detected.")
                return [
                    self._safe_read("/etc/machine-id"),
                    self._safe_read("/var/lib/dbus/machine-id"),
                    self._run("cat /sys/class/dmi/id/product_uuid"),
                ]
            elif self.system == "Windows":
                logger.debug("Windows detected.")
                return [
                    self._run("wmic csproduct get uuid"),
                    self._run("powershell -command \"Get-WmiObject Win32_ComputerSystemProduct | Select-Object -ExpandProperty UUID\"")
                ]
            elif self.system == "Darwin":
                logger.debug("Darwin detected.")
                return [
                    self._run("ioreg -rd1 -c IOPlatformExpertDevice | awk '/IOPlatformUUID/ { print $3; }'").strip('"')
                ]
            else:
                logger.error("Unsupported system: %s", self.system)
                return ["unknown-os"]
        except Exception as e:
            return [str(e)]

    def _safe_read(self, path):
        try:
            with open(path, "r") as f:
                return f.read().strip()
        except Exception:
            return ""

    def _run(self, command):
        try:
            result = subprocess.check_output(command, shell=True, stderr=subprocess.DEVNULL)
            return result.decode().splitlines()[-1].strip()
        except Exception:
            return ""

if __name__ == "__main__":
    node_id = NodeIDGenerator().get_node_id()
    print(node_id)

def get_node_id():
    return NodeIDGenerator().get_node_id()