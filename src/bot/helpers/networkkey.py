import hashlib

def get_network_key(version):
    combine = f"atsuko-nexus-p2p-{version}"
    NETWORK_KEY = hashlib.sha256(combine.encode()).hexdigest()
    return NETWORK_KEY
