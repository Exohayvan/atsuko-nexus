import asyncio
import json
import socket
import time
from kademlia.network import Server
import logging
from helpers.clogging import setup_logging
from helpers.nodeid import get_node_id

setup_logging()
logger = logging.getLogger("P2P")
loggerudp = logging.getLogger("UDP")
loggerdht = logging.getLogger("DHT")

# === Configuration ===
DHT_PORT = 8468         # for kademlia protocol
PING_PORT = 42101       # for raw UDP ping/pong
seen_peers = set()

# === Determine local IP (best effort)
def get_local_ip():
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        s.connect(("8.8.8.8", 80))
        ip = s.getsockname()[0]
        s.close()
        return ip
    except Exception:
        return "0.0.0.0"

# === UDP Ping Logic
async def udp_ping(ip, port=PING_PORT):
    try:
        loop = asyncio.get_running_loop()
        transport, _ = await loop.create_datagram_endpoint(
            lambda: asyncio.DatagramProtocol(), remote_addr=(ip, port)
        )
        start = time.time()
        transport.sendto(b'PING')
        await asyncio.sleep(0.1)
        transport.close()
        logger.info(f"[→] Ping to {ip}:{port} — {(time.time() - start)*1000:.2f} ms")
    except Exception as e:
        logger.error(f"[!] Ping error to {ip}:{port}: {e}")

# === UDP Pong Listener
class PongServer(asyncio.DatagramProtocol):
    def connection_made(self, transport):
        self.transport = transport

    def datagram_received(self, data, addr):
        if data == b'PING':
            self.transport.sendto(b'PONG', addr)
        elif data == b'PONG':
            logger.debug(f"[✓] Pong from {addr}")

# === Main Logic
async def main(NETWORK_KEY):
    loop = asyncio.get_event_loop()
    local_ip = get_local_ip()

    # Start UDP ping server
    transport, _ = await loop.create_datagram_endpoint(
        PongServer, local_addr=("0.0.0.0", PING_PORT)
    )
    loggerudp.debug(f"Listening on {local_ip}:{PING_PORT}")

    # Start DHT node (no bootstrap)
    dht = Server()
    await dht.listen(DHT_PORT)
    loggerdht.debug(f"Running on port {DHT_PORT} without bootstrap")

    self_peer = {"ip": local_ip, "port": PING_PORT}
    encoded_self = json.dumps([self_peer])

    # Initial self-advertise
    await dht.set(NETWORK_KEY, encoded_self)

    while True:
        try:
            value = await dht.get(NETWORK_KEY)
            if not value:
                loggerdht.info("No peers yet.")
                await asyncio.sleep(10)
                continue

            try:
                peers = json.loads(value)
            except json.JSONDecodeError:
                loggerdht.warning("Invalid peer list")
                peers = []

            for peer in peers:
                ip = peer.get("ip")
                port = peer.get("port")
                if (ip, port) not in seen_peers and ip != local_ip:
                    seen_peers.add((ip, port))
                    loggerdht.info(f"Found new peer: {ip}:{port}")
                    await udp_ping(ip, port)

            # Re-publish own presence
            await dht.set(NETWORK_KEY, encoded_self)
            await asyncio.sleep(15)

        except Exception as e:
            logger.error(f"[ERROR] Main loop: {e}")
            await asyncio.sleep(10)

if __name__ == "__main__":
    KEY = "TEST"
    asyncio.run(main(KEY))