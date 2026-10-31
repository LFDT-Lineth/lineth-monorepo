"""Block truncation (§3.2): the stripped transactions and senders of accepted blocks, and the blocks it rejects."""

from dataclasses import replace

import pytest
from ethereum_rlp import rlp
from ethereum_types.bytes import Bytes
from ethereum_types.numeric import U64, U256, Uint

from rollup_spec.fork import (
    AccessListTransaction,
    BlobTransaction,
    FeeMarketTransaction,
    LegacyTransaction,
    SetCodeTransaction,
)
from rollup_spec.rollup import truncate_block_rlp
from tests.blocks import (
    CHAIN_ID,
    RECIPIENT,
    SENDER,
    block,
    block_rlp,
    legacy_tx,
    sign,
    typed_tx,
    wire,
)

# The signed transaction and sender of the EIP-155 specification's example (chain ID 1), and its stripped form.
EIP_155_EXAMPLE = bytes.fromhex(
    "f86c098504a817c800825208943535353535353535353535353535353535353535880de0b6b3a76400008025a028ef61340bd939bc2195fe537567866003e1a15d3c71ff63e1590620aa636276a067cbe9d8997f761aecb703304b3800ccf555c9f3dc64214b297fb1966a3b6d83"
)
EIP_155_SENDER = bytes.fromhex("9d8a62f656a8d1615c1294fd71e9cfb3e4855a4f")
EIP_155_STRIPPED = bytes.fromhex(
    "e9098504a817c800825208943535353535353535353535353535353535353535880de0b6b3a764000080"
)

TRANSACTION_KINDS = [AccessListTransaction, FeeMarketTransaction, BlobTransaction, SetCodeTransaction]


def test_eip_155_example_is_stripped_and_its_sender_recovered():
    example = rlp.decode_to(LegacyTransaction, EIP_155_EXAMPLE)

    truncated = truncate_block_rlp(block_rlp([wire(example)]), U64(1))

    assert truncated.transactions == [EIP_155_STRIPPED]
    assert truncated.froms == [EIP_155_SENDER]


def test_block_of_every_transaction_type_keeps_its_transactions_in_order_with_their_sender():
    transactions = [sign(legacy_tx(0))] + [sign(typed_tx(kind, nonce=i + 1)) for i, kind in enumerate(TRANSACTION_KINDS)]

    truncated = truncate_block_rlp(block_rlp([wire(tx) for tx in transactions]), CHAIN_ID)

    assert truncated.froms == [SENDER] * len(transactions)
    assert [stripped[0] for stripped in truncated.transactions[1:]] == [1, 2, 3, 4]
    assert truncated.transactions[0][0] >= 0xC0
    assert truncated.transactions[0] == rlp.encode([Uint(0), Uint(7), Uint(21_000), RECIPIENT, Uint(1), b""])


def test_block_without_transactions_is_accepted():
    truncated = truncate_block_rlp(block_rlp([]), CHAIN_ID)

    assert (truncated.transactions, truncated.froms) == ([], [])


def envelope(tx) -> Bytes:
    return Bytes(wire(tx))


def swapped_body() -> bytes:
    """The header of a block with one transaction over the body of another transaction by the same sender."""
    genuine = block([wire(sign(legacy_tx(1)))])
    return rlp.encode(replace(genuine, transactions=(wire(sign(legacy_tx(2))),)))


def dropped_transaction() -> bytes:
    """The header of a block with two transactions over a body that holds the first only."""
    first, second = wire(sign(legacy_tx(1))), wire(sign(legacy_tx(2)))
    return rlp.encode(replace(block([first, second]), transactions=(first,)))


def set_code_with_nine_byte_nonce() -> Bytes:
    fields = [Uint(CHAIN_ID), Uint(2**64), Uint(1), Uint(9), Uint(100_000), RECIPIENT, Uint(0), b"", [], [], Uint(0), Uint(1), Uint(1)]
    return Bytes(b"\x04" + rlp.encode(fields))


def unsigned_fee_market_envelope_of_type(type_byte: int) -> Bytes:
    return Bytes(bytes([type_byte]) + wire(sign(typed_tx(FeeMarketTransaction)))[1:])


REJECTED_BLOCKS = [
    pytest.param(
        lambda: block_rlp([wire(sign(legacy_tx()))]) + b"\xde\xad", "block RLP is not canonical",
        id="bytes after the block",
    ),
    pytest.param(
        lambda: block_rlp([Bytes(rlp.encode(sign(legacy_tx())))]), "not a canonical typed transaction",
        id="legacy transaction wrapped as a byte string",
    ),
    pytest.param(
        lambda: block_rlp([Bytes(wire(sign(typed_tx(FeeMarketTransaction))) + b"junk")]), "not a canonical typed transaction",
        id="bytes after a typed transaction",
    ),
    pytest.param(
        lambda: block_rlp([unsigned_fee_market_envelope_of_type(5)]), "not a canonical typed transaction",
        id="transaction type beyond 4",
    ),
    pytest.param(
        lambda: block_rlp([set_code_with_nine_byte_nonce()]), "SetCodeTransaction.nonce",
        id="nine-byte nonce of a set-code transaction",
    ),
    pytest.param(
        lambda: block_rlp([wire(sign(legacy_tx(2**64)))]), "exceeds the width", id="legacy nonce of 2^64",
    ),
    pytest.param(
        lambda: block_rlp([wire(sign(typed_tx(FeeMarketTransaction, nonce=2**64)))]), "exceeds the width",
        id="typed nonce of 2^64",
    ),
    pytest.param(
        lambda: block_rlp([wire(sign(legacy_tx(gas=Uint(2**64))))]), "exceeds the width", id="gas of 2^64",
    ),
    pytest.param(
        lambda: block_rlp([wire(sign(legacy_tx(gas_price=Uint(2**128))))]), "exceeds the width", id="legacy gas price of 2^128",
    ),
    pytest.param(
        lambda: block_rlp([wire(sign(typed_tx(FeeMarketTransaction, max_fee_per_gas=Uint(2**128))))]), "exceeds the width",
        id="max fee of 2^128",
    ),
    pytest.param(
        lambda: block_rlp([wire(sign(typed_tx(FeeMarketTransaction, max_priority_fee_per_gas=Uint(2**128))))]),
        "exceeds the width", id="max priority fee of 2^128",
    ),
    pytest.param(
        lambda: block_rlp([wire(sign(typed_tx(BlobTransaction, max_fee_per_blob_gas=U256(2**128))))]), "exceeds the width",
        id="max blob fee of 2^128",
    ),
    pytest.param(
        lambda: block_rlp([wire(sign(typed_tx(BlobTransaction, blob_versioned_hashes=())))]), "names no blob",
        id="blob transaction without blobs",
    ),
    pytest.param(swapped_body, "transactions root", id="transaction swapped for another one by the same sender"),
    pytest.param(dropped_transaction, "transactions root", id="transaction missing from the body"),
]


@pytest.mark.parametrize("build_block, error", REJECTED_BLOCKS)
def test_block_that_is_not_canonical_or_does_not_carry_its_committed_transactions_is_rejected(build_block, error):
    with pytest.raises(Exception, match=error):
        truncate_block_rlp(build_block(), CHAIN_ID)


def test_largest_nonce_and_fees_the_guest_decodes_are_accepted():
    largest = [
        sign(legacy_tx(2**64 - 1, gas=Uint(2**64 - 1), gas_price=Uint(2**128 - 1))),
        sign(typed_tx(BlobTransaction, nonce=2**64 - 1, max_fee_per_gas=Uint(2**128 - 1), max_priority_fee_per_gas=Uint(2**128 - 1), max_fee_per_blob_gas=U256(2**128 - 1))),
    ]

    truncated = truncate_block_rlp(block_rlp([wire(tx) for tx in largest]), CHAIN_ID)

    assert truncated.froms == [SENDER, SENDER]


def test_chain_id_zero_is_rejected():
    with pytest.raises(Exception, match="chain ID must be positive"):
        truncate_block_rlp(block_rlp([wire(sign(legacy_tx()))]), U64(0))


def test_eip_155_signature_of_another_chain_is_rejected():
    with pytest.raises(Exception):
        truncate_block_rlp(block_rlp([wire(sign(legacy_tx(), chain_id=U64(1)))]), CHAIN_ID)
