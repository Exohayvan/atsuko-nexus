import os
import sys
import logging
import asyncio
import re
import subprocess
import time
import requests
from datetime import datetime
from rich.text import Text
from textual.app import App, ComposeResult
from textual.widgets import Static
from textual.containers import ScrollableContainer, Vertical
from textual.reactive import reactive

from helpers.clogging import setup_logging
from helpers.nodeid import get_node_id

# === Logging Setup ===
setup_logging()
logger = logging.getLogger("MAIN")
loggerup = logging.getLogger("UPDATER")
loggerheart = logging.getLogger("HEARTBEAT")

# === Hardcoded Configs ===
CURRENT_VERSION = "0.0.8-alpha"
REPO = "Exohayvan/atsuko-nexus"
NODE_ID = get_node_id()
LOG_FILE = "./runtime.log"
start_time = datetime.now()
peers = {}

# === Updater Settings ===
import platform
UPDATER_EXE = "updater.exe" if platform.system() == "Windows" else "updater"

# === Config Settings ===
import json
if getattr(sys, 'frozen', False):
    BASE_DIR = os.path.dirname(sys.executable)
else:
    BASE_DIR = os.path.dirname(os.path.abspath(__file__))
CONFIG_FILE = os.path.join(BASE_DIR, "config.json")


# === Load/Create Config ===
def load_or_create_config():
    default_config = {
        "log_level": "INFO"
    }

    if not os.path.exists(CONFIG_FILE):
        with open(CONFIG_FILE, "w", encoding="utf-8") as f:
            json.dump(default_config, f, indent=4)
        return default_config
    else:
        try:
            with open(CONFIG_FILE, "r", encoding="utf-8") as f:
                config = json.load(f)
            for key in default_config:
                if key not in config:
                    config[key] = default_config[key]
            return config
        except Exception as e:
            print(f"Error reading config.json: {e}")
            return default_config

def load_or_create_config():
    default_config = {
        "log_level": "INFO",
        "display_levels": ["INFO", "WARNING", "ERROR", "CRITICAL"]
    }

    if not os.path.exists(CONFIG_FILE):
        with open(CONFIG_FILE, "w", encoding="utf-8") as f:
            json.dump(default_config, f, indent=4)
        return default_config
    else:
        try:
            with open(CONFIG_FILE, "r", encoding="utf-8") as f:
                config = json.load(f)
            # Fill in missing keys
            for key in default_config:
                if key not in config:
                    config[key] = default_config[key]
            return config
        except Exception as e:
            print(f"Error reading config.json: {e}")
            return default_config

async def get_latest_release_tag():
    api_url = f"https://api.github.com/repos/{REPO}/releases/latest"
    try:
        response = requests.get(api_url, timeout=5)
        if response.status_code == 200:
            return response.json()["tag_name"]
        else:
            while response.status_code != 200:
                response = requests.get(api_url, timeout=5)
                if response.status_code == 200:
                    return response.json()["tag_name"]
                else:
                    loggerup.warning(f"Failed to fetch release info for {REPO}. Status code: {response.status_code}")
                    loggerup.info("Rechecking for update in 10 seconds...")
                    await asyncio.sleep(10)
            loggerup.warning(f"Failed to fetch release info for {REPO}. Status code: {response.status_code}")
    except Exception as e:
        loggerup.error(f"Error during update check: {e}")
    return None

async def check_for_update_and_run_updater():
    loggerup.info("Checking for latest version...")
    latest_tag = await get_latest_release_tag()
    loggerup.debug(f"Latest GitHub version: {latest_tag}")
    loggerup.debug(f"Current Local version: {CURRENT_VERSION}")

    if latest_tag and latest_tag != CURRENT_VERSION:
        loggerup.info(f"New version found: {latest_tag} (current: {CURRENT_VERSION})")
        loggerup.info("Downloading and installing latest version in 5 seconds...")
        loggerup.info(f"If update fails, please run the updater program. Or download manually at https://github.com/{REPO}/releases/latest/download/atsuko-nexus.exe")
        await asyncio.sleep(5)
        download_url = f"https://github.com/{REPO}/releases/latest/download/atsuko-nexus.exe"
        updater_path = os.path.join(os.path.dirname(sys.argv[0]), UPDATER_EXE)
        
        print(f"🔧 Updater path: {updater_path}")
        print(f"📥 Download URL: {download_url}")

        try:
            subprocess.Popen([updater_path, sys.argv[0], download_url])
        except FileNotFoundError as e:
            loggerup.error(f"Failed to launch updater: {e}")
        except Exception as e:
            loggerup.error(f"Unexpected error: {e}")
        time.sleep(1)
        sys.exit(0)
    else:
        loggerup.info(f"No update found (current: {CURRENT_VERSION})")
    time.sleep(1)

# === TUI Widgets ===
class StatusBar(Static):
    def on_mount(self):
        self.set_interval(1, self.update_status)

    def update_status(self):
        uptime = datetime.now() - start_time
        uptime_str = str(uptime).split(".")[0]
        self.update(
            f"[b cyan]🆔 Node ID:[/b cyan] {NODE_ID}   "
            f"[b green]⏱️ Uptime:[/b green] {uptime_str}   "
            f"[b blue]📊 Version:[/b blue] {CURRENT_VERSION}   "
            f"[b red] 🔴 Peers:[/b red] {len(peers)}"
        )

class LogViewer(Static):
    lines = reactive([])
    logloaded = False

    def on_mount(self):
        self.set_interval(0.5, self.refresh_log)

    def refresh_log(self):
        try:
            with open(LOG_FILE, "r", encoding="utf-8") as f:
                self.lines = f.readlines()[-200:]
        except FileNotFoundError:
            self.lines = ["Waiting for runtime.log..."]

        self.update(self.render_log())
        container = self.app.query_one("#log_viewer").parent
        if hasattr(container, "scroll_end"):
            container.scroll_end(animate=False)

    def render_log(self) -> Text:
        rendered = Text()

        try:
            display_config = load_or_create_config()
            display_levels = display_config["display_levels"]
            if self.logloaded == False:
                logger.debug(f"Loaded Logs with display_levels: {display_levels}")
            self.logloaded=True
        except Exception as e:
            logger.error(f"Failed to load config for log display: {e}")
            display_levels = []  # Show nothing if config is broken

        for line in self.lines:
            line = line.strip()
            match = re.match(r"^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3}):(\w+):([^:]+): (.*)", line)
            if match:
                raw_ts, level, logger_name, message = match.groups()

                if level.upper() not in display_levels:
                    continue

                try:
                    dt = datetime.strptime(raw_ts, "%Y-%m-%d %H:%M:%S,%f")
                    ts_display = dt.strftime("%Y.%m.%d %H:%M:%S")
                except Exception:
                    ts_display = raw_ts

                level_styles = {
                    "DEBUG": "bright_blue",
                    "INFO": "green",
                    "WARNING": "yellow",
                    "ERROR": "red",
                    "CRITICAL": "bold red"
                }
                level_color = level_styles.get(level.upper(), "white")

                rendered.append(f"{ts_display}", style="dim")
                rendered.append(" | ")
                rendered.append(f"{level}", style=level_color)
                rendered.append(" | ")
                rendered.append(f"{logger_name}", style="cyan")
                rendered.append(" | ")
                rendered.append(f"{message}\n", style="white")
            else:
                rendered.append(line + "\n", style="white")

        return rendered

class Divider(Static):
    def render(self) -> Text:
        return Text("─" * self.app.size.width, style="dim")

# === Main App ===
class DashboardApp(App):
    CSS_PATH = None
    BINDINGS = [("q", "quit", "Quit")]

    def compose(self) -> ComposeResult:
        yield Vertical(
            Static("💠 [b magenta]Atsuko Nexus Status Monitor[/b magenta]", classes="title"),
            StatusBar(),
            Divider(),
            Static("[b yellow]Live Logs (runtime.log):[/b yellow]"),
            ScrollableContainer(LogViewer(id="log_viewer"))
        )

    def on_ready(self):
        asyncio.create_task(self.background_task())

    async def background_task(self):
        logger.debug(f"Working directory: {os.getcwd()}")
        logger.debug(f"Node ID: {NODE_ID}")
        while True:
            try:
                await check_for_update_and_run_updater()
                loggerheart.info("Node still alive.")
                await asyncio.sleep(30)
            except Exception as e:
                logger.error(f"Error: {e}")
                logger.warning("Script restarting in 10 seconds...")
                await asyncio.sleep(10)

# === Entry Point ===
if __name__ == "__main__":
    logger.debug("Main script Loaded. Logging started...")
    config = load_or_create_config()
    DashboardApp().run()