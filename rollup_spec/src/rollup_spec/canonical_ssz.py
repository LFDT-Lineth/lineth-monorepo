"""
Canonical consensus-layer SSZ containers, copied verbatim from consensus-specs
@ v1.6.1 so they track upstream as forks evolve. They use `remerkleable` (the
library consensus-specs generates against), so the class bodies are identical to
upstream — only the imports and `source:` comments are local.

To re-sync: bump the tag, re-copy the affected `class`/alias/constant bodies from
the referenced spec files, and re-run the stateless-input SSZ decode round-trip
(`stateless_input.py::decode_stateless_input_ssz`, guarded by `_strict_decode`).

Only canonical CL containers live here; the bespoke EIP-8025 stateless-input
envelope is in `stateless_input.py`.
"""

from remerkleable.basic import uint64, uint256
from remerkleable.byte_arrays import ByteList, ByteVector, Bytes32, Bytes48, Bytes96
from remerkleable.complex import Container, List

# ── Preset constants ─────────────────────────────────────────────────────────
# source: presets/mainnet/bellatrix.yaml
#   https://github.com/ethereum/consensus-specs/blob/v1.6.1/presets/mainnet/bellatrix.yaml
MAX_BYTES_PER_TRANSACTION = 2**30                # 1073741824
MAX_TRANSACTIONS_PER_PAYLOAD = 2**20             # 1048576
BYTES_PER_LOGS_BLOOM = 2**8                      # 256
MAX_EXTRA_DATA_BYTES = 2**5                      # 32
# source: presets/mainnet/capella.yaml
#   https://github.com/ethereum/consensus-specs/blob/v1.6.1/presets/mainnet/capella.yaml
MAX_WITHDRAWALS_PER_PAYLOAD = 2**4               # 16
# source: presets/mainnet/electra.yaml
#   https://github.com/ethereum/consensus-specs/blob/v1.6.1/presets/mainnet/electra.yaml
MAX_DEPOSIT_REQUESTS_PER_PAYLOAD = 2**13         # 8192
MAX_WITHDRAWAL_REQUESTS_PER_PAYLOAD = 2**4       # 16
MAX_CONSOLIDATION_REQUESTS_PER_PAYLOAD = 2**1    # 2

# ── Custom type aliases ──────────────────────────────────────────────────────
# source: specs/phase0/beacon-chain.md  (Custom types table)
#   https://github.com/ethereum/consensus-specs/blob/v1.6.1/specs/phase0/beacon-chain.md
Hash32 = Bytes32
Gwei = uint64
ValidatorIndex = uint64
BLSPubkey = Bytes48
BLSSignature = Bytes96
# source: specs/capella/beacon-chain.md  (Custom types table)
#   https://github.com/ethereum/consensus-specs/blob/v1.6.1/specs/capella/beacon-chain.md
WithdrawalIndex = uint64
# source: specs/bellatrix/beacon-chain.md  (Custom types table)
#   https://github.com/ethereum/consensus-specs/blob/v1.6.1/specs/bellatrix/beacon-chain.md
ExecutionAddress = ByteVector[20]  # Bytes20
Transaction = ByteList[MAX_BYTES_PER_TRANSACTION]

# ── Containers (verbatim) ────────────────────────────────────────────────────
# source: specs/capella/beacon-chain.md
#   https://github.com/ethereum/consensus-specs/blob/v1.6.1/specs/capella/beacon-chain.md


class Withdrawal(Container):
    index: WithdrawalIndex
    validator_index: ValidatorIndex
    address: ExecutionAddress
    amount: Gwei


# source: specs/deneb/beacon-chain.md  (unchanged through Electra/Fulu @ v1.6.1)
#   https://github.com/ethereum/consensus-specs/blob/v1.6.1/specs/deneb/beacon-chain.md


class ExecutionPayload(Container):
    parent_hash: Hash32
    fee_recipient: ExecutionAddress
    state_root: Bytes32
    receipts_root: Bytes32
    logs_bloom: ByteVector[BYTES_PER_LOGS_BLOOM]
    prev_randao: Bytes32
    block_number: uint64
    gas_limit: uint64
    gas_used: uint64
    timestamp: uint64
    extra_data: ByteList[MAX_EXTRA_DATA_BYTES]
    base_fee_per_gas: uint256
    block_hash: Hash32
    transactions: List[Transaction, MAX_TRANSACTIONS_PER_PAYLOAD]
    withdrawals: List[Withdrawal, MAX_WITHDRAWALS_PER_PAYLOAD]
    # [New in Deneb:EIP4844]
    blob_gas_used: uint64
    # [New in Deneb:EIP4844]
    excess_blob_gas: uint64


# source: specs/electra/beacon-chain.md
#   https://github.com/ethereum/consensus-specs/blob/v1.6.1/specs/electra/beacon-chain.md


class DepositRequest(Container):
    pubkey: BLSPubkey
    withdrawal_credentials: Bytes32
    amount: Gwei
    signature: BLSSignature
    index: uint64


class WithdrawalRequest(Container):
    source_address: ExecutionAddress
    validator_pubkey: BLSPubkey
    amount: Gwei


class ConsolidationRequest(Container):
    source_address: ExecutionAddress
    source_pubkey: BLSPubkey
    target_pubkey: BLSPubkey


class ExecutionRequests(Container):
    # [New in Electra:EIP6110]
    deposits: List[DepositRequest, MAX_DEPOSIT_REQUESTS_PER_PAYLOAD]
    # [New in Electra:EIP7002:EIP7251]
    withdrawals: List[WithdrawalRequest, MAX_WITHDRAWAL_REQUESTS_PER_PAYLOAD]
    # [New in Electra:EIP7251]
    consolidations: List[ConsolidationRequest, MAX_CONSOLIDATION_REQUESTS_PER_PAYLOAD]
