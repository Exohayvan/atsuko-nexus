import os
import logging
import asyncio
import re
from datetime import datetime
from rich.text import Text
from rich.style import Style
from rich.markup import escape
from textual.app import App, ComposeResult
from textual.widgets import Static
from textual.containers import Vertical, VerticalScroll
from textual.reactive import reactive
from textual.widget import Widget

from helpers.clogging import setup_logging
from helpers.nodeid import get_node_id

# === Config ===
NODE_ID = get_node_id()
LOG_FILE = "./runtime.log"
start_time = datetime.now()

# === TUI Widgets ===
class StatusBar(Static):
    def on_mount(self):
        self.set_interval(1, self.update_status)

    def update_status(self):
        uptime = datetime.now() - start_time
        uptime_str = str(uptime).split(".")[0]
        self.update(
            f"[b cyan]🆔 Node ID:[/b cyan] {NODE_ID}   "
            f"[b green]⏱️ Uptime:[/b green] {uptime_str}"
        )

class LogViewer(Widget):
    lines = reactive([])

    def on_mount(self):
        self.set_interval(0.5, self.refresh_log)

    def refresh_log(self):
        try:
            with open(LOG_FILE, "r", encoding="utf-8") as f:
                self.lines = f.readlines()[-200:]  # Show more lines
        except FileNotFoundError:
            self.lines = ["Waiting for runtime.log..."]

    def render(self):
        rendered = Text()

        for line in self.lines:
            line = line.strip()

            # Try to match log line with format: TIMESTAMP:LEVEL:LOGGER: MESSAGE
            match = re.match(r"^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3}):(\w+):([^:]+): (.*)", line)

            if match:
                raw_ts, level, logger, message = match.groups()

                # Convert timestamp
                try:
                    dt = datetime.strptime(raw_ts, "%Y-%m-%d %H:%M:%S,%f")
                    ts_display = dt.strftime("%Y.%m.%d %H:%M:%S")
                except Exception:
                    ts_display = raw_ts  # fallback if malformed

                # Log level colors
                level_styles = {
                    "DEBUG": "bright_blue",
                    "INFO": "green",
                    "WARNING": "yellow",
                    "ERROR": "red",
                    "CRITICAL": "bold red"
                }
                level_color = level_styles.get(level.upper(), "white")

                # Render line
                rendered.append(f"{ts_display}", style="dim")
                rendered.append(" | ")
                rendered.append(f"{level}", style=level_color)
                rendered.append(" | ")
                rendered.append(f"{logger}", style="cyan")
                rendered.append(" | ")
                rendered.append(f"{message}\n", style="white")

            else:
                # Fallback for any non-log lines
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
            VerticalScroll(LogViewer())
        )

    def on_ready(self):
        asyncio.create_task(self.background_task())

    async def background_task(self):
        logger = logging.getLogger("MAIN")
        logger.debug("Main script Loaded. Logging started...")
        logger.debug(f"Working directory: {os.getcwd()}")
        logger.debug(f"Node ID: {NODE_ID}")

        while True:
            logger.info("Heartbeat: node still alive.")
            await asyncio.sleep(10)

# === Entry Point ===
if __name__ == "__main__":
    setup_logging()
    DashboardApp().run()
