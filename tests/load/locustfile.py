import random
import requests
from locust import HttpUser, task, between, events

def getLatestHeight(layer):
    return requests.get(f"https://index.oasislabs.com/v1/{layer}/status").json()['latest_block']

def getLatestTxs(layer, n):
    txs = requests.get(f"https://index.oasislabs.com/v1/{layer}/transactions?limit={n}").json()['transactions']
    return [tx['hash'] for tx in txs]

def getSampleAddrs(layer, n):
    addrs = set()
    offset = 0
    while len(addrs) < n:
        print(f"addr length: {len(addrs)}, offset: {offset}")
        txs = requests.get(f"https://index.oasislabs.com/v1/{layer}/transactions?limit={n}&offset={offset}").json()['transactions']
        addrs.update({tx['sender_0'] for tx in txs})
        addrs.update({tx['to'] for tx in txs if 'to' in tx})
        offset += 100
    
    return list(addrs)[:n]

numSamples = 200
EmeraldHeight = getLatestHeight("emerald")
SapphireHeight = getLatestHeight("sapphire")
EmeraldTxs = getLatestTxs("emerald", numSamples)
SapphireTxs = getLatestTxs("sapphire", numSamples)
EmeraldAddrs = getSampleAddrs("emerald", numSamples)
# The early history of sapphire only consists of the Oasis tx_client, 
# which means there are only 2 active addresses.
# SapphireAddrs = getSampleAddrs("sapphire", numSamples)

@events.init_command_line_parser.add_listener
def _(parser):
    parser.add_argument("--paratime", type=str, env_var="LOCUST_PARATIME", default="emerald", help="The lowercase name of the paratime to query")

class RuntimeUser(HttpUser):
    wait_time = between(1, 5)

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.paratime = self.environment.parsed_options.paratime

    # https://explorer.dev.oasis.io/
    @task(5)
    def homepage(self):
        self.client.get("/v1/")
    
    # https://explorer.dev.oasis.io/mainnet/emerald
    @task(6)
    def paratime(self):
        self.client.get("/v1/")
        self.client.get(f"/v1/{self.paratime}/status")
        self.client.get(f"/v1/{self.paratime}/stats/tx_volume?bucket_size_seconds=300&limit=288")
        self.client.get(f"/v1/{self.paratime}/stats/active_accounts?limit=288&window_step_seconds=300")
        self.client.get(f"/v1/{self.paratime}/stats/active_accounts?limit=7&window_step_seconds=86400")
        self.client.get(f"/v1/{self.paratime}/stats/active_accounts?limit=30&window_step_seconds=86400")
        self.client.get(f"/v1/{self.paratime}/transactions?limit=5")
        self.client.get(f"/v1/{self.paratime}/blocks?limit=5")
        self.client.get(f"/v1/{self.paratime}/evm_tokens?limit=5")
        self.client.get(f"/v1/{self.paratime}/stats/tx_volume?bucket_size_seconds=86400&limit=30")

    # https://explorer.dev.oasis.io/mainnet/emerald/tx/{hash}
    @task(4)
    def tx_detail(self):
        if self.paratime == "emerald":
            tx_hash = EmeraldTxs[random.randint(0, numSamples-1)]
        else:
            tx_hash = SapphireTxs[random.randint(0, numSamples-1)]
        
        self.client.get("/v1/")
        self.client.get(f"/v1/{self.paratime}/status")
        self.client.get(f"/v1/{self.paratime}/transactions/{tx_hash}", name=f"/v1/{self.paratime}/transactions/hash")
        self.client.get(f"/v1/{self.paratime}/events?tx_hash={tx_hash}&limit=100", name=f"/v1/{self.paratime}/events/hash")
    
    # https://explorer.dev.oasis.io/mainnet/emerald/address/{address}
    @task(4)
    def account_detail(self):
        if self.paratime == "emerald":
            addr = EmeraldAddrs[random.randint(0, numSamples-1)]
        else:
            addr = SapphireAddrs[random.randint(0, numSamples-1)]
        
        self.client.get("/v1/")
        self.client.get(f"/v1/{self.paratime}/status")
        self.client.get(f"/v1/{self.paratime}/transactions?rel={addr}&limit=10&offset=0", name=f"v1/{self.paratime}/transactions?rel=addr")
        self.client.get(f"/v1/{self.paratime}/accounts/{addr}", name=f"/v1/{self.paratime}/accounts/addr")

    # https://explorer.dev.oasis.io/mainnet/emerald/block/{height}
    @task(4)
    def block_detail(self):
        if self.paratime == "emerald":
            height = random.randint(1, EmeraldHeight)
        else:
            height = random.randint(1, SapphireHeight)
        
        self.client.get("/v1/")
        self.client.get(f"/v1/{self.paratime}/status")
        self.client.get(f"/v1/{self.paratime}/blocks?to={height}&limit=1", name=f"/v1/{self.paratime}/blocks/height")
        self.client.get(f"/v1/{self.paratime}/transactions?block={height}&limit=10&offset=0", name=f"/v1/{self.paratime}/transactions/?block=height")

    # https://explorer.dev.oasis.io/mainnet/emerald/block
    @task(2)
    def blocks(self):
        self.client.get("/v1/")
        self.client.get(f"/v1/{self.paratime}/status")
        self.client.get(f"/v1/{self.paratime}/blocks?limit=10&offset=0")

    # https://explorer.dev.oasis.io/mainnet/emerald/tx
    @task(2)
    def txs(self):
        self.client.get("/v1/")
        self.client.get(f"/v1/{self.paratime}/status")
        self.client.get(f"/v1/{self.paratime}/transactions?limit=10&offset=0")

    # https://explorer.dev.oasis.io/mainnet/emerald/token
    @task(2)
    def tokens(self):
        self.client.get("/v1/")
        self.client.get(f"/v1/{self.paratime}/status")
        self.client.get(f"/v1/{self.paratime}/evm_tokens?limit=10&offset=0")
