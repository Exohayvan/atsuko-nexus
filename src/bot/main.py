import os
import discord
from discord.ext import commands
import asyncio
import logging
import json
from clogging import setup_logging

# Initialize logging
setup_logging()
logger = logging.getLogger('main.py')
logger.debug("------------------------------------------------------------------------")
logger.debug("Main script Loaded. Logging started...")

logger.debug(f"Current working directory: {os.getcwd()}")

intents = discord.Intents.all()

async def determine_prefix(bot, message):
    return '!'

def get_config():
    base_path = os.path.dirname(os.path.abspath(__file__))
    config_path = os.path.join(base_path, "../../../config.json")
    with open(config_path, 'r') as f:
        return json.load(f)

bot = commands.Bot(command_prefix=determine_prefix, intents=intents, owner_id=276782057412362241, case_insensitive=True)

async def load_cogs(bot, root_dir):
    logger.debug(f"Loading cogs from: {root_dir}")
    tasks = []
    num_cogs = 0
    
    # Load all cogs
    for dirpath, dirnames, filenames in os.walk(root_dir):
        for filename in filenames:
            if filename.endswith('.py'):
                await handle_loading(dirpath, filename, bot, tasks)
                num_cogs += 1

    await asyncio.gather(*tasks)
    return num_cogs

async def handle_loading(dirpath, filename, bot, tasks):
    path = os.path.join(dirpath, filename)
    module = path.replace(os.sep, ".")[:-3]
    try:
        task = asyncio.create_task(bot.load_extension(module))
        tasks.append(task)
        logger.debug(f"Loaded Cog: {module}")
    except Exception as e:
        logger.error(f"Failed to load Cog: {module}\n{e}")

@bot.event
async def on_ready():
    logger.debug(f"Logged in as {bot.user}")
    logger.debug("Loading cogs...")
    num_cogs = await load_cogs(bot, 'commands')
    logger.info(f"Cogs loaded ({num_cogs} cogs)")
    await bot.tree.sync()

@bot.event
async def on_message(message):
    if message.author.bot:
        return

    await bot.process_commands(message)

config = get_config()
bot.run(config['bot_token'])