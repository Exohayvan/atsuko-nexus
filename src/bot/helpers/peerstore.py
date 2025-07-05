import asyncio

peers = set()
peers_lock = asyncio.Lock()  # For thread-safe updates
