"""Builders for signed transactions and Amsterdam blocks whose body is the one their header commits to."""

from dataclasses import replace
from typing import Sequence

from coincurve import PrivateKey
from ethereum.crypto.hash import Hash32, keccak256
from ethereum.forks.amsterdam.transactions import (
    signing_hash_155,
    signing_hash_1559,
    signing_hash_2930,
    signing_hash_4844,
    signing_hash_7702,
)
from ethereum.merkle_patricia_trie import Trie, root, trie_set
from ethereum.state import Address
from ethereum_rlp import rlp
from ethereum_types.bytes import Bytes, Bytes8, Bytes20, Bytes32, Bytes256
from ethereum_types.numeric import U64, U256, Uint

from rollup_spec.fork import (
    AccessListTransaction,
    Block,
    BlobTransaction,
    FeeMarketTransaction,
    Header,
    LegacyTransaction,
    SetCodeTransaction,
    Transaction,
    encode_transaction,
)

CHAIN_ID = U64(59144)
SECRET_KEY = bytes([0x42]) * 32
SENDER = Address(keccak256(PrivateKey(SECRET_KEY).public_key.format(compressed=False)[1:])[12:])

ZERO = Hash32(bytes(32))
RECIPIENT = Bytes20(b"\x11" * 20)
BLOB_HASH = Hash32(b"\x01" + b"\x22" * 31)


def legacy_tx(nonce: int = 0, **fields) -> LegacyTransaction:
    """An unsigned legacy transaction; `fields` override any of its fields."""
    base = dict(
        nonce=U256(nonce), gas_price=Uint(7), gas=Uint(21_000), to=RECIPIENT, value=U256(1), data=b"",
        v=U256(0), r=U256(0), s=U256(0),
    )
    return LegacyTransaction(**{**base, **fields})


def typed_tx(kind: type, nonce: int = 0, **fields) -> Transaction:
    """An unsigned transaction of the typed `kind`; `fields` override any of its fields."""
    common = dict(
        chain_id=CHAIN_ID, nonce=U256(nonce), gas=Uint(100_000), to=RECIPIENT, value=U256(0), data=b"\x01\x02",
        y_parity=U256(0), r=U256(0), s=U256(0),
    )
    fees = dict(max_priority_fee_per_gas=Uint(1), max_fee_per_gas=Uint(9))
    specific = {
        AccessListTransaction: dict(gas_price=Uint(7), access_list=()),
        FeeMarketTransaction: dict(**fees, access_list=()),
        BlobTransaction: dict(
            **fees, access_list=(), max_fee_per_blob_gas=U256(5), blob_versioned_hashes=(BLOB_HASH,),
        ),
        SetCodeTransaction: dict(**fees, access_list=(), authorizations=()),
    }[kind]
    if kind is SetCodeTransaction:
        common["nonce"] = U64(nonce)
    return kind(**{**common, **specific, **fields})


def sign(tx: Transaction, chain_id: U64 = CHAIN_ID) -> Transaction:
    """`tx` signed with SECRET_KEY; a legacy transaction carries an EIP-155 signature for `chain_id`."""
    digest = {
        LegacyTransaction: lambda: signing_hash_155(tx, chain_id),
        AccessListTransaction: lambda: signing_hash_2930(tx),
        FeeMarketTransaction: lambda: signing_hash_1559(tx),
        BlobTransaction: lambda: signing_hash_4844(tx),
        SetCodeTransaction: lambda: signing_hash_7702(tx),
    }[type(tx)]()
    signature = PrivateKey(SECRET_KEY).sign_recoverable(digest, hasher=None)
    r, s = U256(int.from_bytes(signature[:32], "big")), U256(int.from_bytes(signature[32:64], "big"))
    if isinstance(tx, LegacyTransaction):
        return replace(tx, v=U256(signature[64] + 35 + 2 * int(chain_id)), r=r, s=s)
    return replace(tx, y_parity=U256(signature[64]), r=r, s=s)


def wire(tx: Transaction) -> Transaction | Bytes:
    """The entry of a block's transaction list for `tx`: a legacy transaction, or a typed one's envelope."""
    return encode_transaction(tx)


def transactions_root(items: Sequence[Transaction | Bytes]) -> Hash32:
    """The transactions trie root of a block body whose transaction list is `items`."""
    trie = Trie(secured=False, default=None)
    for index, item in enumerate(items):
        trie_set(trie, rlp.encode(Uint(index)), item)
    return root(trie)


def header(number: int = 1, parent_hash: Hash32 = ZERO, root_of_transactions: Hash32 = ZERO) -> Header:
    zero = Bytes32(bytes(32))
    return Header(
        parent_hash=parent_hash, ommers_hash=zero, coinbase=Bytes20(bytes(20)), state_root=zero,
        transactions_root=root_of_transactions, receipt_root=zero, bloom=Bytes256(bytes(256)), difficulty=Uint(0),
        number=Uint(number), gas_limit=Uint(30_000_000), gas_used=Uint(21_000),
        timestamp=U256(1_700_000_000 + number), extra_data=b"", prev_randao=zero, nonce=Bytes8(bytes(8)),
        base_fee_per_gas=Uint(7), withdrawals_root=zero, blob_gas_used=U64(0), excess_blob_gas=U64(0),
        parent_beacon_block_root=zero, requests_hash=zero, block_access_list_hash=zero, slot_number=U64(number),
    )


def block(items: Sequence[Transaction | Bytes], number: int = 1, parent_hash: Hash32 = ZERO) -> Block:
    """The block of `number` whose transaction list is `items` and whose header commits to exactly them."""
    block_header = header(number, parent_hash, transactions_root(items))
    return Block(header=block_header, transactions=tuple(items), ommers=(), withdrawals=())


def block_rlp(items: Sequence[Transaction | Bytes], number: int = 1, parent_hash: Hash32 = ZERO) -> bytes:
    return rlp.encode(block(items, number, parent_hash))
