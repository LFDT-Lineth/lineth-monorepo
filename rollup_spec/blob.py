from ethereum.forks.osaka.blocks import Block as EthereumBlock
from dataclasses import dataclass
from ethereum_types.numeric import U64
from ethereum.crypto.hash import Hash32, keccak256
from ethereum_types.bytes import Bytes32, Bytes48
from ethereum.state import Address
from typing import List, Tuple

from ethereum_rlp import rlp
from ethereum.forks.osaka.transactions import (
    LegacyTransaction,
    AccessListTransaction,
    FeeMarketTransaction,
    BlobTransaction,
    SetCodeTransaction,
    Transaction,
    recover_sender,
)

from ethereum.crypto.kzg import (
    KZGCommitment,
    kzg_commitment_to_versioned_hash,
    verify_kzg_proof,
)
from .block import block_hash

@dataclass
class TruncatedEthereumBlock:
    """
    TruncatedEthereumBlock is the truncated content of an EthereumBlock, it
    corresponds to what is publicly exposed to the L1 contract
    """
    timestamp: U64
    block_hash: Hash32
    prev_randao: Bytes32
    transactions: List[bytes]
    froms: List[Address]

    def is_consistent_with(self, other: EthereumBlock, chain_id: U64) -> bool:
        """
        is_consistent_with checks if a truncated block is consistent with its
        full version.
        """
        num_transactions = len(self.transactions)
        if num_transactions != len(other.transactions):
            return False
        if num_transactions != len(self.froms):
            return False
        if self.timestamp != other.header.timestamp:
            return False 
        if self.prev_randao != other.header.prev_randao:
            return False
        if self.block_hash != block_hash(other.header):
            return False
        for i in range(num_transactions):
            tx_rlp = self.transactions[i]
            tx = other.transactions[i]
            if encode_as_signing_rlp(tx, chain_id) != tx_rlp:
                return False
            recovered_from = recover_sender(chain_id, tx)
            if recovered_from != self.froms[i]:
                return False
        return True

@dataclass
class ShnarfWitness:
    """
    ShnarfWitness is the preimage of a shnarf
    """
    prev_shnarf: Hash32
    block_hash: Hash32
    blob_hash: Hash32

    def hash(self) -> Hash32:
        return keccak256(self.prev_shnarf + self.block_hash + self.blob_hash)

@dataclass
class RollupDataWitness:
    """
    RollupSubmittedData represents the witness of a data-submission; e.g. the
    inputs of the prover relative to a particular blob submission.
    """
    blob_bytes: bytes
    block_number_range: tuple[int, int]
    blob_kzg_commitment: KZGCommitment
    blob_kzg_proof: Bytes48

    def blob_hash(self) -> bytes:
        """
        blob_hash returns the versioned hash of the blob KZG commitment.
        """
        return kzg_commitment_to_versioned_hash(self.blob_kzg_commitment)
    
    def is_authenticated_blob_bytes(self) -> Tuple[bool, Hash32] :
        """
        is_authenticated_blob_bytes checks the KZG proofs and returns true if
        it successfully validates that the blob_hash relates to the blob_data.

        The function also returns the blob_hash.
        """
        blob_hash = self.blob_hash()
        blob_kzg_x = keccak256(self.blob_bytes + blob_hash)
        blob_poly = parse_as_bls12_381_fr_vec(self.blob_bytes)
        blob_kzg_y = eval_lagrange_bls12_381(blob_poly, blob_kzg_x)
        return verify_kzg_proof(self.blob_kzg_commitment, blob_kzg_x, blob_kzg_y, self.blob_kzg_proof), blob_hash
    
    def parse_block_data(self) -> List[TruncatedEthereumBlock]:
        """
        This parts parses the blob-data and checks the relevant execution
        proof for each transaction.
        """
        blob_data_uncompressed = decompress_lz4(self.blob_bytes)
        return parse_public_da_block_data(blob_data_uncompressed)


def encode_as_signing_rlp(tx: Transaction, chain_id: U64) -> bytes:
    """
    encode_as_signing_rlp returns the unsigned RLP encoding of a transaction, as used in
    Linea's DA format. Each transaction type signs a different preimage; this
    function returns that.
    """
    if isinstance(tx, LegacyTransaction):
        # Pre-EIP-155 transactions use v ∈ {27, 28}; their signing preimage is
        # the 6-field tuple (nonce, gas_price, gas, to, value, data).
        # EIP-155 transactions encode chain_id into v as chain_id * 2 + {35, 36}
        # and sign the 9-field tuple (..., chain_id, 0, 0).
        if tx.v in (27, 28):
            return rlp.encode(
                (
                    tx.nonce,
                    tx.gas_price,
                    tx.gas,
                    tx.to,
                    tx.value,
                    tx.data,
                )
            )
        return rlp.encode(
            (
                tx.nonce,
                tx.gas_price,
                tx.gas,
                tx.to,
                tx.value,
                tx.data,
                chain_id,
                0,
                0,
            )
        )
    if isinstance(tx, AccessListTransaction):
        return b"\x01" + rlp.encode(
            (
                tx.chain_id,
                tx.nonce,
                tx.gas_price,
                tx.gas,
                tx.to,
                tx.value,
                tx.data,
                tx.access_list,
            )
        )
    if isinstance(tx, FeeMarketTransaction):
        return b"\x02" + rlp.encode(
            (
                tx.chain_id,
                tx.nonce,
                tx.max_priority_fee_per_gas,
                tx.max_fee_per_gas,
                tx.gas,
                tx.to,
                tx.value,
                tx.data,
                tx.access_list, 
            )
        )
    if isinstance(tx, BlobTransaction):
        return b"\x03" + rlp.encode(
            (
                tx.chain_id,
                tx.nonce,
                tx.max_priority_fee_per_gas,
                tx.max_fee_per_gas,
                tx.gas,
                tx.to,
                tx.value,
                tx.data,
                tx.access_list,
                tx.max_fee_per_blob_gas,
                tx.blob_versioned_hashes, 
            )
        )
    if isinstance(tx, SetCodeTransaction):
        return b"\x04" + rlp.encode(
            (
                tx.chain_id,
                tx.nonce,
                tx.max_priority_fee_per_gas,
                tx.max_fee_per_gas,
                tx.gas,
                tx.to,
                tx.value,
                tx.data,
                tx.access_list,
                tx.authorizations,
            )
        )
    raise Exception("unknown type of transaction")


def decompress_lz4(data: bytes) -> bytes:
    """
    decompress_lz4 decompressed with LZ4 decompresses and returns the bytes
    """
    raise NotImplementedError("decompress_lz4")

def parse_public_da_block_data(data: bytes) -> List[TruncatedEthereumBlock]:
    """
    parse_public_da_block_data parses the blockdata coming from a DA blob. The
    encoding of the block data relies on the RLP encoding of the block.
    """
    raise NotImplementedError("parse_public_da_block_data")

def parse_as_bls12_381_fr_vec(data: bytes) -> List[Bytes32]:
    """
    parse_as_bls12_381_fr_vec parses the input sequence of bytes into a sequence
    of BLS12-381 field elements. The function must return an error if the input
    cannot be parsed into an array of BLS12-381 field elements.

    The way the function works is that it expects data to be formed as a
    sequence of BLS12-381 scalar field elements all stored over 32 bytes. So
    the function does not really do anything aside slicing the input data and
    checking the field elements are well formed (don't overflow the modulus)
    """
    raise NotImplementedError("parse_as_bls12_381_fr_vec")

def eval_lagrange_bls12_381(poly: List[Bytes32], x: bytes) -> bytes:
    """
    eval_lagrange_bls12_381 evaluates a polynomial in Lagrange form in the
    BLS12-381 field
    """
    raise NotImplementedError("eval_lagrange_bls12_381")
