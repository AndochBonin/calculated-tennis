import os

from dotenv import load_dotenv
from py_clob_client_v2 import ClobClient

from load_secrets import must_load_from_env_if_configured

load_dotenv()
must_load_from_env_if_configured()

metamask_key = os.getenv("METAMASK_KEY")


def _signer_address():
    if not metamask_key:
        return None
    try:
        from eth_account import Account

        return Account.from_key(metamask_key).address
    except Exception:
        return None


client = ClobClient(
    host="https://clob.polymarket.com",
    chain_id=137,
    key=metamask_key,
)

addr = _signer_address()
if addr:
    print("Signer address (use as POLYMARKET_ADDRESS in Go):", addr)

credentials = client.derive_api_key()

print(credentials)
