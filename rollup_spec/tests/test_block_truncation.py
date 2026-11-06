"""Block truncation (§3.2): the stripped transactions and senders of accepted blocks, and the blocks it rejects."""

from dataclasses import replace

import pytest
from ethereum.exceptions import InvalidSignatureError
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
    header,
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

TRANSACTION_KINDS = [AccessListTransaction, FeeMarketTransaction, SetCodeTransaction]


def test_eip_155_example_is_stripped_and_its_sender_recovered():
    example = rlp.decode_to(LegacyTransaction, EIP_155_EXAMPLE)

    truncated = truncate_block_rlp(block_rlp([wire(example)]), U64(1))

    assert truncated.transactions == [EIP_155_STRIPPED]
    assert truncated.froms == [EIP_155_SENDER]


def test_block_of_every_transaction_type_keeps_its_transactions_in_order_with_their_sender():
    transactions = [sign(legacy_tx(0))] + [sign(typed_tx(kind, nonce=i + 1)) for i, kind in enumerate(TRANSACTION_KINDS)]

    truncated = truncate_block_rlp(block_rlp([wire(tx) for tx in transactions]), CHAIN_ID)

    assert truncated.froms == [SENDER] * len(transactions)
    assert [stripped[0] for stripped in truncated.transactions[1:]] == [1, 2, 4]
    assert truncated.transactions[0][0] >= 0xC0
    assert truncated.transactions[0] == rlp.encode([Uint(0), Uint(7), Uint(21_000), RECIPIENT, Uint(1), b""])


def test_block_without_transactions_is_accepted():
    truncated = truncate_block_rlp(block_rlp([]), CHAIN_ID)

    assert (truncated.transactions, truncated.froms) == ([], [])


def test_chain_id_zero_is_rejected_even_for_a_block_without_transactions():
    with pytest.raises(Exception, match="positive chain ID"):
        truncate_block_rlp(block_rlp([]), U64(0))


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


def legacy_with_v(v: int) -> LegacyTransaction:
    """A legacy transaction under a valid r and s whose `v` is `v`."""
    return replace(sign(legacy_tx()), v=U256(v))


# Neither 27, 28 nor 35 or 36 above twice the chain ID: values below the EIP-155 range, the neighbours of the chain's values, a
# chain ID beyond 64 bits, and the widest 256-bit integer.
INVALID_LEGACY_V = [0, 1, 26, 29, 34, 2 * int(CHAIN_ID) + 34, 2 * int(CHAIN_ID) + 37, 2 * 2**64 + 35, 2**256 - 1]


def unsigned_fee_market_envelope_of_type(type_byte: int) -> Bytes:
    return Bytes(bytes([type_byte]) + wire(sign(typed_tx(FeeMarketTransaction)))[1:])


UNSUPPORTED_TYPE = "neither a legacy list nor an envelope of type 1, 2 or 4"

REJECTED_BLOCKS = [
    pytest.param(
        lambda: block_rlp([Bytes(rlp.encode(sign(legacy_tx())))]), UNSUPPORTED_TYPE,
        id="legacy transaction wrapped as a byte string",
    ),
    pytest.param(
        lambda: block_rlp([unsigned_fee_market_envelope_of_type(5)]), UNSUPPORTED_TYPE,
        id="transaction type beyond 4",
    ),
    pytest.param(
        lambda: block_rlp([envelope(sign(typed_tx(BlobTransaction)))]), UNSUPPORTED_TYPE,
        id="blob transaction",
    ),
    pytest.param(
        lambda: block_rlp([wire(sign(legacy_tx(0))), envelope(sign(typed_tx(BlobTransaction, nonce=1)))]),
        UNSUPPORTED_TYPE, id="blob transaction after a legacy one",
    ),
    pytest.param(
        lambda: block_rlp([set_code_with_nine_byte_nonce()]), "SetCodeTransaction.nonce",
        id="nine-byte nonce of a set-code transaction",
    ),
    pytest.param(swapped_body, "transactions root", id="transaction swapped for another one by the same sender"),
    pytest.param(dropped_transaction, "transactions root", id="transaction missing from the body"),
    *[
        pytest.param(lambda v=v: block_rlp([wire(legacy_with_v(v))]), "bad v", id=f"legacy v of {v}")
        for v in INVALID_LEGACY_V
    ],
]


@pytest.mark.parametrize("build_block, error", REJECTED_BLOCKS)
def test_block_with_an_unsupported_or_uncommitted_transaction_is_rejected(build_block, error):
    with pytest.raises(Exception, match=error):
        truncate_block_rlp(build_block(), CHAIN_ID)


def test_ommer_timestamp_is_a_256_bit_integer_and_the_block_timestamp_a_64_bit_one():
    genuine = block([wire(sign(legacy_tx(1)))])
    for timestamp in [2**64, 2**256 - 1]:
        with_ommer = replace(genuine, ommers=(replace(header(5), timestamp=U256(timestamp)),))

        assert truncate_block_rlp(rlp.encode(with_ommer), CHAIN_ID) == truncate_block_rlp(rlp.encode(genuine), CHAIN_ID)

    beyond_64_bits = replace(genuine, header=replace(genuine.header, timestamp=U256(2**64)))
    with pytest.raises(OverflowError):
        truncate_block_rlp(rlp.encode(beyond_64_bits), CHAIN_ID)


def test_eip_155_signature_of_another_chain_is_rejected():
    other_chain = block_rlp([wire(sign(legacy_tx(), chain_id=U64(1)))])

    assert truncate_block_rlp(other_chain, U64(1)).froms == [SENDER]
    with pytest.raises(InvalidSignatureError):
        truncate_block_rlp(other_chain, CHAIN_ID)
