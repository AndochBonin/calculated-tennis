import os
import inspect

from dotenv import load_dotenv
from py_clob_client_v2 import ClobClient, SignatureTypeV2
from py_builder_relayer_client.client import RelayClient
from py_builder_signing_sdk.config import BuilderApiKeyCreds, BuilderConfig

load_dotenv()

print(inspect.signature(RelayClient.__init__))

builder_config = BuilderConfig(
    local_builder_creds=BuilderApiKeyCreds(
        key=os.environ["BUILDER_API_KEY"],
        secret=os.environ["BUILDER_SECRET"],
        passphrase=os.environ["BUILDER_PASSPHRASE"],
    )
)

relayer = RelayClient(
    os.environ["RELAYER_URL"],
    int(os.environ.get("CHAIN_ID", "137")),
    os.environ["METAMASK_KEY"],
    builder_config,
)

print(f"key={os.environ['BUILDER_API_KEY']}")
print(f"relayer={os.environ['RELAYER_URL']}")

deposit_wallet = relayer.get_expected_safe()
print(f"DEPOSIT_WALLET={deposit_wallet}")

response = relayer.deploy()
confirmed = relayer.poll_until_state(response, "STATE_CONFIRMED", "STATE_FAILED")
print("Wallet deployed successfully")