"""Select a representative set of accepted blocks from the EF zkevm blockchain-test corpus.

Prints one line per pick, `path | case | block index | criterion`, with `path` relative to the
corpus directory. The selection is deterministic: fixture files are visited smallest first, ties
broken by path.

Run: python3 -I select_ef_blocks.py [corpus_dir]
The corpus directory defaults to ~/.cache/lineth/zkevm-fixtures/blockchain_tests/for_amsterdam.
"""

import json
import os
import sys
import zlib
from dataclasses import dataclass

DEFAULT_CORPUS = os.path.expanduser("~/.cache/lineth/zkevm-fixtures/blockchain_tests/for_amsterdam")

BLOB_TYPE = 3
# A block RLP whose zlib size is at least this fraction of its length compresses poorly,
# at most the second fraction compresses well.
POOR_RATIO = 0.85
GOOD_RATIO = 0.2
CHAIN_MIN_LENGTH = 8
# Blocks above this RLP size are not picked (the corpus holds blocks at the 8 MiB protocol limit).
MAX_RLP_LEN = 1 << 20
# Chain preferred when it still qualifies.
PREFERRED_CHAIN = (
    "cancun/eip4788_beacon_root/beacon_root_contract/multi_block_beacon_root_timestamp_calls.json",
    "tests/cancun/eip4788_beacon_root/test_beacon_root_contract.py::test_multi_block_beacon_root_timestamp_calls"
    "[fork_Amsterdam-blockchain_test-block_count_10-buffer_wraparound]",
)


@dataclass
class Block:
    path: str
    case: str
    index: int
    tx_count: int
    types: frozenset
    rlp_len: int
    zlib_len: int
    has_access_list: bool
    has_creation: bool
    has_authorization: bool

    @property
    def key(self):
        return (self.path, self.case, self.index)

    @property
    def ratio(self):
        return self.zlib_len / self.rlp_len


def tx_type(tx):
    return int(tx.get("type", "0x0"), 16)


def is_accepted(block):
    return "expectException" not in block and "blockHeader" in block


def scan(corpus):
    files = []
    for root, _, names in os.walk(corpus):
        for name in names:
            if name.endswith(".json"):
                full = os.path.join(root, name)
                files.append((os.path.getsize(full), os.path.relpath(full, corpus)))
    files.sort()
    blocks = []
    chains = []
    for _, rel in files:
        with open(os.path.join(corpus, rel)) as handle:
            document = json.load(handle)
        for case in sorted(document):
            raw_blocks = document[case].get("blocks", [])
            run = []
            for index, raw in enumerate(raw_blocks):
                if not is_accepted(raw):
                    close_run(chains, rel, case, run)
                    run = []
                    continue
                txs = raw.get("transactions", [])
                rlp = bytes.fromhex(raw["rlp"][2:])
                block = Block(
                    path=rel,
                    case=case,
                    index=index,
                    tx_count=len(txs),
                    types=frozenset(tx_type(t) for t in txs),
                    rlp_len=len(rlp),
                    zlib_len=len(zlib.compress(rlp, 9)),
                    has_access_list=any(t.get("accessList") for t in txs),
                    has_creation=any(not t.get("to") for t in txs),
                    has_authorization=any(t.get("authorizationList") for t in txs),
                )
                blocks.append(block)
                linked = run and raw["blockHeader"]["parentHash"] == run[-1][1]
                if run and not linked:
                    close_run(chains, rel, case, run)
                    run = []
                if txs and BLOB_TYPE not in block.types:
                    run.append((block, raw["blockHeader"]["hash"]))
                else:
                    close_run(chains, rel, case, run)
                    run = []
            close_run(chains, rel, case, run)
    return blocks, chains


def close_run(chains, rel, case, run):
    if len(run) >= CHAIN_MIN_LENGTH:
        chains.append((rel, case, run[0][0].index, len(run)))


def smallest_file_first(candidates):
    return min(candidates, key=lambda b: (os.path.getsize(os.path.join(CORPUS, b.path)), b.path, b.case, b.index), default=None)


def main():
    global CORPUS
    CORPUS = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_CORPUS
    blocks, chains = scan(CORPUS)
    eligible = [b for b in blocks if b.tx_count >= 2 and BLOB_TYPE not in b.types and b.rlp_len <= MAX_RLP_LEN]

    picks = {}

    def add(block, criterion):
        if block is None:
            return
        picks.setdefault(block.key, (block, []))[1].append(criterion)

    def add_feature(flag, criterion):
        candidates = [b for b in eligible if flag(b)]
        chosen = [b for b in candidates if b.key in picks]
        add(smallest_file_first(chosen) or smallest_file_first(candidates), criterion)

    type_sets = sorted({b.types for b in eligible}, key=lambda s: (len(s), sorted(s)))
    for types in type_sets:
        label = "+".join(f"type{t}" for t in sorted(types))
        add(smallest_file_first([b for b in eligible if b.types == types]), f"transaction types {label}")

    add_feature(lambda b: b.has_access_list, "access list")
    add_feature(lambda b: b.has_creation, "contract creation")
    add_feature(lambda b: b.has_authorization, "EIP-7702 authorization list")

    most = max(b.tx_count for b in eligible)
    add(smallest_file_first([b for b in eligible if b.tx_count == most]), f"most transactions ({most})")
    poor = [b for b in eligible if b.ratio >= POOR_RATIO]
    big_poor = max(b.rlp_len for b in poor)
    add(smallest_file_first([b for b in poor if b.rlp_len == big_poor]), f"largest poorly compressing RLP ({big_poor} bytes, zlib ratio >= {POOR_RATIO})")
    good = [b for b in eligible if b.ratio <= GOOD_RATIO]
    big_good = max(b.rlp_len for b in good)
    add(smallest_file_first([b for b in good if b.rlp_len == big_good]), f"largest well compressing RLP ({big_good} bytes, zlib ratio <= {GOOD_RATIO})")

    blob = [b for b in blocks if BLOB_TYPE in b.types]
    add(smallest_file_first(blob), "accepted block with a blob transaction")

    preferred = [c for c in chains if (c[0], c[1]) == PREFERRED_CHAIN]
    chain = preferred[0] if preferred else min(chains, key=lambda c: (os.path.getsize(os.path.join(CORPUS, c[0])), c[0], c[1], c[2]))
    lines = [f"{b.path} | {b.case} | {b.index} | {'; '.join(c)}" for b, c in picks.values()]
    lines.append(f"{chain[0]} | {chain[1]} | {chain[2]} | chain of {chain[3]} consecutive accepted blocks with a non-blob transaction")

    sys.stdout.write("\n".join(lines) + "\n")


if __name__ == "__main__":
    main()
