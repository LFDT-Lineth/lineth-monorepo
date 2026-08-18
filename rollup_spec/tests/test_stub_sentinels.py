"""
Independently verifies the rollup / rollup-aggregation stub guests' sentinel constants
(`riscv-guests/rollup/src/rollup.zig`, `riscv-guests/rollup-aggregation/src/rollup_aggregation.zig`).

Each sentinel is `keccak256(tag)` for a documented tag string; Zig has no keccak available at
comptime or in a native test (the guest's own keccak is a runtime accelerator opcode), so the hex
is pinned in the Zig source rather than derived there. This test recomputes keccak256 of each tag
with a library implementation and asserts it against the same hex, so a wrong or stale sentinel
fails a test instead of relying on a one-off computation nobody re-checks.

If a sentinel changes on the Zig side, update the matching entry here — this is a deliberate
cross-check, not a discoverability pointer: the two sides are independent computations of the same
value, not one file describing another.

Run from the rollup_spec/ directory:  python -m pytest tests/test_stub_sentinels.py
"""

import pytest

from ethereum.crypto.hash import keccak256

# (tag, expected hex, byte length). A 32-byte sentinel is the full hash; an 8-byte sentinel (a u64)
# is the hash's first 8 bytes, big-endian.
_SENTINELS = [
    (
        "lineth.stub.rollup.l2L1BridgeTransactionTree",
        "bc436fcfbb175835d12e0a12f4534f60cd92dbd4babf87db2339277a72ecd22a",
        32,
    ),
    (
        "lineth.stub.rollup.endDataRollingHash",
        "1fe617c10b3bfc97fd6d0090c608df47de544a4b7d4f6379300bd96167da2ada",
        32,
    ),
    (
        "lineth.stub.rollup.filteredAddressesHash",
        "8fa4b00e95cd0784a49f00efe0d12f67715a99a6cf54ac4ca5606ebc4d0a42ba",
        32,
    ),
    ("lineth.stub.rollup.endOffset", "1ab5956f53caf2ea", 8),
    (
        "lineth.stub.rollup.l2L1Roots",
        "45c25758659787f96843b2171dd2091c964ee7fe11518fabf1a4c944b4f75e0e",
        32,
    ),
    (
        "lineth.stub.rollup-aggregation.l2L1BridgeTransactionTree",
        "0918836198239a5edf0936db3e28a64d5c4e195fd728e6ddfc3544fc95008ab3",
        32,
    ),
    (
        "lineth.stub.rollup-aggregation.filteredAddressesHash",
        "63d40d3ea387065027b369a25cc441dd7685e2a9a9e0e56556ec2e82bbe2273c",
        32,
    ),
    (
        "lineth.stub.rollup-aggregation.l2MessagingBlocksOffsets",
        "d18d873fe2a9f192",
        8,
    ),
]


@pytest.mark.parametrize("tag, expected_hex, byte_length", _SENTINELS, ids=[s[0] for s in _SENTINELS])
def test_stub_sentinel_matches_keccak256_of_its_tag(tag: str, expected_hex: str, byte_length: int) -> None:
    digest = keccak256(tag.encode("ascii"))[:byte_length]
    assert digest == bytes.fromhex(expected_hex)
