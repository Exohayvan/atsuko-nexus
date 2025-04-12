import os
import discord
from discord.ext import commands
import asyncio
import logging
import json
from helpers.clogging import setup_logging
from helpers.nodeid import get_node_id

# Initialize logging
setup_logging()
logger = logging.getLogger('main.py')
logger.debug("------------------------------------------------------------------------")
logger.debug("Main script Loaded. Logging started...")

logger.debug(f"Current working directory: {os.getcwd()}")

nodeID = get_node_id()
print(nodeID)