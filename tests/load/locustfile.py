"""
This file contains the load test for simulating usage of the paratime
Explorer pages, currently Emerald and Sapphire. The basic 
flow begins in startup() which fetches the latest height, recent 
transactions, and recent addresses from the paratime. These are then
queried during the load test. 

The core of the load test is the `RuntimeUser` defined below, which 
simulates usage of the Oasis Explorer. Each method annotated with `@task`
simulates all the calls to Nexus for a given Explorer page. During the load
test, each simulated user picks a task chosen at random based on the 
task weights `@task(weight)`. The pages are currently weighted unequally
based on expected usage of the Explorer. After the task completes, the user
waits a random duration defined in `RuntimeUser.wait_time` (currently 
between 1-5 seconds) before starting another random task.
"""

import random
import requests
from locust import HttpUser, task, between, events

@events.init_command_line_parser.add_listener
def _(parser):
    parser.add_argument("--paratime", type=str, env_var="LOCUST_PARATIME", default="emerald", help="The lowercase name of the paratime to query")
    parser.add_argument("--sample-pool-size", type=int, env_var="LOCUST_SAMPLE_POOL_SIZE", default=100, help="The n most recent blocks/txs/addrs to query during the load test")

@events.test_start.add_listener
def startup(environment, **kwargs):
    baseUrl = environment.parsed_options.host
    paratime = environment.parsed_options.paratime
    numSamples = environment.parsed_options.sample_pool_size
    print(f"baseUrl: {baseUrl}, paratime: {paratime}, numSamples: {numSamples}")

    global CurrHeight, TestAddrs, TestTxs
    CurrHeight = getLatestHeight(baseUrl, paratime)
    TestTxs = getLatestTxs(baseUrl, paratime, numSamples)
    TestAddrs = getSampleAddrs(baseUrl, paratime, numSamples)

def getLatestHeight(baseUrl, layer):
    return requests.get(f"{baseUrl}/v1/{layer}/status").json()['latest_block']

def getLatestTxs(baseUrl, layer, n):
    txs = requests.get(f"{baseUrl}/v1/{layer}/transactions?limit={n}").json()['transactions']
    return [tx['hash'] for tx in txs]

def getSampleAddrs(baseUrl, layer, n):
    addrs = set()
    offset = 0
    while len(addrs) < n:
        print(f"total addrs found: {len(addrs)}, offset: {offset}")
        txs = requests.get(f"{baseUrl}/v1/{layer}/transactions?limit={n}&offset={offset}").json()['transactions']
        addrs.update({tx['sender_0'] for tx in txs})
        addrs.update({tx['to'] for tx in txs if 'to' in tx})
        offset += 100
    
    return list(addrs)[:n]


class RuntimeUser(HttpUser):
    wait_time = between(1, 5)

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.paratime = self.environment.parsed_options.paratime
        self.numSamples = self.environment.parsed_options.sample_pool_size

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
        tx_hash = TestTxs[random.randint(0, self.numSamples-1)]

        self.client.get("/v1/")
        self.client.get(f"/v1/{self.paratime}/status")
        self.client.get(f"/v1/{self.paratime}/transactions/{tx_hash}", name=f"/v1/{self.paratime}/transactions/hash")
        self.client.get(f"/v1/{self.paratime}/events?tx_hash={tx_hash}&limit=100", name=f"/v1/{self.paratime}/events/hash")
    
    # https://explorer.dev.oasis.io/mainnet/emerald/address/{address}
    @task(4)
    def account_detail(self):
        addr = TestAddrs[random.randint(0, self.numSamples-1)]

        self.client.get("/v1/")
        self.client.get(f"/v1/{self.paratime}/status")
        self.client.get(f"/v1/{self.paratime}/transactions?rel={addr}&limit=10&offset=0", name=f"v1/{self.paratime}/transactions?rel=addr")
        self.client.get(f"/v1/{self.paratime}/accounts/{addr}", name=f"/v1/{self.paratime}/accounts/addr")

    # https://explorer.dev.oasis.io/mainnet/emerald/block/{height}
    @task(4)
    def block_detail(self):
        height = random.randint(1, CurrHeight)

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
