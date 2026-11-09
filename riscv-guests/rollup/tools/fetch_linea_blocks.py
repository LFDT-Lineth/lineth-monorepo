"""Build a Linea mainnet block dataset in the EF blockchain-test JSON shape.

`fetch` (the default) downloads the blocks WINDOW_FIRST..WINDOW_LAST from the public RPC with full
transaction objects, rebuilds every header and transaction as RLP, checks each block against the
hash and `transactionsRoot` the RPC reports, gives every header the two fields Amsterdam adds to
the Prague header (the hash of an empty block access list and slot number 0) with each parent hash
relinked to the hash of the reshaped block before it, and writes
`../test/testdata/linea_mainnet_<first>_<last>.json.gz`: one case with `config.chainid` 0xe708 and
one entry per block holding `rlp`, `blockHeader` (hash, parentHash, mixHash, number, timestamp) and
`transactions`. The hashes of the file are those of the reshaped headers, except the first block's
parent hash. The gzip stream carries no filename and a zero mtime, so a rerun reproduces the file
byte for byte.

`discover` samples block transaction counts through the first week of January 2026 and prints the
densest window of WINDOW_BLOCKS consecutive blocks, to be pasted into the constants below.

Run with the rollup_spec virtualenv:
    rollup_spec/.venv/bin/python fetch_linea_blocks.py [fetch|discover]
"""

import gzip
import json
import os
import sys
import time
import urllib.error
import urllib.request
import zlib
from collections import Counter

from ethereum.crypto.hash import keccak256
from ethereum.forks.prague.blocks import Block, Header, Withdrawal
from ethereum.forks.prague.fork_types import Authorization
from ethereum.forks.prague.transactions import (
    AccessListTransaction,
    Access,
    BlobTransaction,
    FeeMarketTransaction,
    LegacyTransaction,
    SetCodeTransaction,
    encode_transaction,
    get_transaction_hash,
)
from ethereum.merkle_patricia_trie import Trie, root, trie_set
from ethereum_rlp import rlp
from ethereum_types.bytes import Bytes, Bytes0, Bytes8, Bytes20, Bytes32, Bytes256
from ethereum_types.numeric import U8, U64, U256, Uint

RPC_URL = "https://rpc.linea.build"
USER_AGENT = "lineth-rollup-testdata/1.0 (block fixture builder for the Lineth monorepo)"
CHAIN_ID = 59144
REQUEST_DELAY_SECONDS = 0.15
MAX_ATTEMPTS = 6

WINDOW_FIRST = 27434569
WINDOW_LAST = 27434718
WINDOW_BLOCKS = 150

WEEK_START = 1767225600  # 2026-01-01T00:00:00Z
WEEK_END = 1767830400  # 2026-01-08T00:00:00Z
COARSE_STRIDE = 600
COARSE_CANDIDATES = 6
FINE_STRIDE = 20
FINE_SPAN = 1500
DENSE_MARGIN = 400

TESTDATA = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "test", "testdata")

_last_request = 0.0


def rpc(method, params):
    global _last_request
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).encode()
    for attempt in range(MAX_ATTEMPTS):
        wait = REQUEST_DELAY_SECONDS - (time.monotonic() - _last_request)
        if wait > 0:
            time.sleep(wait)
        _last_request = time.monotonic()
        request = urllib.request.Request(
            RPC_URL, body, {"Content-Type": "application/json", "User-Agent": USER_AGENT}
        )
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                reply = json.load(response)
            if "error" in reply:
                raise RuntimeError(f"{method}: {reply['error']}")
            if reply.get("result") is None:
                raise RuntimeError(f"{method}{params}: empty result")
            return reply["result"]
        except (urllib.error.URLError, TimeoutError, RuntimeError, json.JSONDecodeError) as error:
            if attempt == MAX_ATTEMPTS - 1:
                raise
            time.sleep(2**attempt)
            print(f"retrying {method}: {error}", file=sys.stderr)


def number_hex(n):
    return hex(n)


def block_header_only(n):
    return rpc("eth_getBlockByNumber", [number_hex(n), False])


def tx_count(n):
    return int(rpc("eth_getBlockTransactionCountByNumber", [number_hex(n)]), 16)


# ── discovery ────────────────────────────────────────────────────────────────────────────────────


def first_block_at_or_after(timestamp, low, high):
    while low < high:
        mid = (low + high) // 2
        if int(block_header_only(mid)["timestamp"], 16) < timestamp:
            low = mid + 1
        else:
            high = mid
    return low


def discover():
    latest = int(rpc("eth_blockNumber", []), 16)
    first = first_block_at_or_after(WEEK_START, 0, latest)
    last = first_block_at_or_after(WEEK_END, first, latest) - 1
    print(f"week: blocks {first}..{last}")
    coarse = [(tx_count(n), n) for n in range(first, last + 1, COARSE_STRIDE)]
    best = sorted(coarse, reverse=True)[:COARSE_CANDIDATES]
    print("coarse top:", best)
    fine = []
    for _, center in best:
        for n in range(max(first, center - FINE_SPAN), min(last, center + FINE_SPAN) + 1, FINE_STRIDE):
            fine.append((tx_count(n), n))
    peak = max(fine)[1]
    lo, hi = max(first, peak - DENSE_MARGIN), min(last, peak + DENSE_MARGIN)
    counts = {n: tx_count(n) for n in range(lo, hi + 1)}
    best_sum, best_start = max(
        (sum(counts[m] for m in range(s, s + WINDOW_BLOCKS)), s) for s in range(lo, hi - WINDOW_BLOCKS + 2)
    )
    end = best_start + WINDOW_BLOCKS - 1
    print(f"densest window: WINDOW_FIRST = {best_start}, WINDOW_LAST = {end}, {best_sum} transactions")


# ── re-encoding ──────────────────────────────────────────────────────────────────────────────────


def q(value):
    return int(value, 16)


def raw(value):
    return bytes.fromhex(value[2:])


def address(value):
    return Bytes20(raw(value))


def recipient(value):
    return Bytes0(b"") if value is None else address(value)


def access_list(items):
    return tuple(Access(account=address(i["address"]), slots=tuple(Bytes32(raw(k)) for k in i["storageKeys"])) for i in items)


def build_transaction(tx):
    kind = q(tx["type"])
    common = dict(nonce=U256(q(tx["nonce"])), gas=Uint(q(tx["gas"])), to=recipient(tx["to"]), value=U256(q(tx["value"])), data=Bytes(raw(tx["input"])))
    signature = dict(r=U256(q(tx["r"])), s=U256(q(tx["s"])))
    if kind == 0:
        return LegacyTransaction(gas_price=Uint(q(tx["gasPrice"])), v=U256(q(tx["v"])), **common, **signature)
    typed = dict(chain_id=U64(q(tx["chainId"])), access_list=access_list(tx["accessList"]), y_parity=U256(q(tx["yParity"])))
    if kind == 1:
        return AccessListTransaction(gas_price=Uint(q(tx["gasPrice"])), **common, **typed, **signature)
    fees = dict(max_priority_fee_per_gas=Uint(q(tx["maxPriorityFeePerGas"])), max_fee_per_gas=Uint(q(tx["maxFeePerGas"])))
    if kind == 2:
        return FeeMarketTransaction(**fees, **common, **typed, **signature)
    if kind == 4:
        authorizations = tuple(
            Authorization(
                chain_id=U256(q(a["chainId"])),
                address=address(a["address"]),
                nonce=U64(q(a["nonce"])),
                y_parity=U8(q(a["yParity"])),
                r=U256(q(a["r"])),
                s=U256(q(a["s"])),
            )
            for a in tx["authorizationList"]
        )
        return SetCodeTransaction(**fees, authorizations=authorizations, **common, **typed, **signature)
    if kind == 3:
        return BlobTransaction(
            **fees,
            max_fee_per_blob_gas=U256(q(tx["maxFeePerBlobGas"])),
            blob_versioned_hashes=tuple(Bytes32(raw(h)) for h in tx["blobVersionedHashes"]),
            **common,
            **typed,
            **signature,
        )
    raise ValueError(f"unsupported transaction type {kind}")


def build_header(block):
    return Header(
        parent_hash=Bytes32(raw(block["parentHash"])),
        ommers_hash=Bytes32(raw(block["sha3Uncles"])),
        coinbase=address(block["miner"]),
        state_root=Bytes32(raw(block["stateRoot"])),
        transactions_root=Bytes32(raw(block["transactionsRoot"])),
        receipt_root=Bytes32(raw(block["receiptsRoot"])),
        bloom=Bytes256(raw(block["logsBloom"])),
        difficulty=Uint(q(block["difficulty"])),
        number=Uint(q(block["number"])),
        gas_limit=Uint(q(block["gasLimit"])),
        gas_used=Uint(q(block["gasUsed"])),
        timestamp=U256(q(block["timestamp"])),
        extra_data=Bytes(raw(block["extraData"])),
        prev_randao=Bytes32(raw(block["mixHash"])),
        nonce=Bytes8(raw(block["nonce"])),
        base_fee_per_gas=Uint(q(block["baseFeePerGas"])),
        withdrawals_root=Bytes32(raw(block["withdrawalsRoot"])),
        blob_gas_used=U64(q(block["blobGasUsed"])),
        excess_blob_gas=U64(q(block["excessBlobGas"])),
        parent_beacon_block_root=Bytes32(raw(block["parentBeaconBlockRoot"])),
        requests_hash=Bytes32(raw(block["requestsHash"])),
    )


def rebuild(block):
    """Returns the block RLP; raises if the rebuilt block differs from what the RPC reports."""
    number = q(block["number"])
    header = build_header(block)
    if "0x" + keccak256(rlp.encode(header)).hex() != block["hash"]:
        raise ValueError(f"block {number}: header hash mismatch")
    transactions = [build_transaction(tx) for tx in block["transactions"]]
    trie = Trie(secured=False, default=None)
    for index, (tx, source) in enumerate(zip(transactions, block["transactions"])):
        if "0x" + get_transaction_hash(encode_transaction(tx)).hex() != source["hash"]:
            raise ValueError(f"block {number}: transaction {index} hash mismatch")
        trie_set(trie, rlp.encode(Uint(index)), encode_transaction(tx))
    if "0x" + root(trie).hex() != block["transactionsRoot"]:
        raise ValueError(f"block {number}: transactions root mismatch")
    withdrawals = tuple(
        Withdrawal(index=U64(q(w["index"])), validator_index=U64(q(w["validatorIndex"])), address=address(w["address"]), amount=U256(q(w["amount"])))
        for w in block["withdrawals"]
    )
    if block["uncles"]:
        raise ValueError(f"block {number}: has ommers")
    encoded = tuple(encode_transaction(tx) for tx in transactions)
    return rlp.encode(Block(header=header, transactions=encoded, ommers=(), withdrawals=withdrawals))


def amsterdam_shaped(block_rlps, parent_hash):
    """The Amsterdam-shaped blocks of consecutive Prague-shaped ones, and their hashes.

    Returns (rlp, hash, parent_hash) per block. The headers are what the guest parses; the first
    block keeps `parent_hash`, and every other block descends from the reshaped block before it.
    """
    empty_block_access_list_hash = keccak256(rlp.encode(()))
    out = []
    for block_rlp in block_rlps:
        header, *body = rlp.decode(block_rlp)
        header[0] = Bytes32(parent_hash)
        header += [Bytes32(empty_block_access_list_hash), Bytes(b"")]
        block_hash = keccak256(rlp.encode(header))
        out.append((rlp.encode([header, *body]), block_hash, parent_hash))
        parent_hash = block_hash
    return out


# ── JSON in the EF shape ─────────────────────────────────────────────────────────────────────────


def quantity(value):
    digits = format(value, "x")
    return "0x" + ("0" + digits if len(digits) % 2 else digits)


def quantity_of(text):
    return quantity(q(text))


def fixture_transaction(tx):
    kind = q(tx["type"])
    out = {"type": quantity(kind), "chainId": quantity_of(tx["chainId"]) if "chainId" in tx else quantity(CHAIN_ID)}
    out["nonce"] = quantity_of(tx["nonce"])
    if kind <= 1:
        out["gasPrice"] = quantity_of(tx["gasPrice"])
    else:
        out["maxPriorityFeePerGas"] = quantity_of(tx["maxPriorityFeePerGas"])
        out["maxFeePerGas"] = quantity_of(tx["maxFeePerGas"])
    out["gasLimit"] = quantity_of(tx["gas"])
    out["to"] = tx["to"] or ""
    out["value"] = quantity_of(tx["value"])
    out["data"] = tx["input"]
    if kind >= 1:
        out["accessList"] = [{"address": i["address"], "storageKeys": i["storageKeys"]} for i in tx["accessList"]]
    if kind == 4:
        out["authorizationList"] = [
            {k: (quantity_of(a[k]) if k in ("chainId", "nonce", "yParity") else a[k]) for k in ("chainId", "address", "nonce", "yParity", "r", "s")}
            for a in tx["authorizationList"]
        ]
    out["v"] = quantity_of(tx["v"])
    out["r"] = tx["r"]
    out["s"] = tx["s"]
    out["sender"] = tx["from"]
    return out


def fixture_block(block, block_rlp, block_hash, parent_hash):
    return {
        "rlp": "0x" + block_rlp.hex(),
        "blockHeader": {
            "hash": "0x" + block_hash.hex(),
            "parentHash": "0x" + parent_hash.hex(),
            "mixHash": block["mixHash"],
            "number": quantity_of(block["number"]),
            "timestamp": quantity_of(block["timestamp"]),
        },
        "transactions": [fixture_transaction(tx) for tx in block["transactions"]],
    }


def fetch():
    if WINDOW_FIRST == 0:
        sys.exit("set WINDOW_FIRST and WINDOW_LAST (run `discover` to find them)")
    sources, rlps, types, fields = [], [], Counter(), set()
    for number in range(WINDOW_FIRST, WINDOW_LAST + 1):
        block = rpc("eth_getBlockByNumber", [number_hex(number), True])
        fields.update(block)
        for tx in block["transactions"]:
            types[q(tx["type"])] += 1
        sources.append(block)
        rlps.append(rebuild(block))
    reshaped = amsterdam_shaped(rlps, raw(sources[0]["parentHash"]))
    blocks = [fixture_block(block, *shaped) for block, shaped in zip(sources, reshaped)]
    rlps = [shaped[0] for shaped in reshaped]
    name = f"linea_mainnet_{WINDOW_FIRST}_{WINDOW_LAST}"
    document = {name: {"config": {"chainid": quantity(CHAIN_ID)}, "blocks": blocks}}
    payload = json.dumps(document, separators=(",", ":")).encode()
    os.makedirs(TESTDATA, exist_ok=True)
    path = os.path.join(TESTDATA, name + ".json.gz")
    with open(path, "wb") as handle:
        handle.write(gzip.compress(payload, compresslevel=9, mtime=0))
    stream = b"".join(rlps)
    print(f"{path}")
    print(f"blocks {len(blocks)}, transactions {sum(types.values())}, type histogram {dict(sorted(types.items()))}")
    print(f"block RLP total {len(stream)} bytes, zlib {len(zlib.compress(stream, 9))} bytes, file {os.path.getsize(path)} bytes")
    print("header fields seen:", sorted(fields))


if __name__ == "__main__":
    command = sys.argv[1] if len(sys.argv) > 1 else "fetch"
    {"fetch": fetch, "discover": discover}[command]()
