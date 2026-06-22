from typing import Any, TypeAlias

from ethereum.crypto.hash import Hash32
from ethereum.state import Address
from ethereum_types.bytes import Bytes32
from ethereum_types.numeric import U64, U256, Uint
from remerkleable.basic import uint64
from remerkleable.byte_arrays import ByteList, ByteVector, Bytes32 as SszBytes32
from remerkleable.complex import Container, List

from . import canonical_ssz as cl
from . import fork
from .fork import Withdrawal
from .block import (
    ConsolidationRequest,
    DepositRequest,
    ExecutionPayload,
    ExecutionRequests,
    NewPayloadRequest,
    StatelessChainConfig,
    StatelessInput,
    WithdrawalRequest,
    decode_signed_transaction_rlp,
)
from .state_transition import ExecutionWitness


# Two-byte big-endian schema id that every stateless input is prefixed with
# (execution-specs `stateless_ssz.py::SCHEMA_ID`). Required by the decoder.
STATELESS_INPUT_SCHEMA_ID = 0x0001
STATELESS_INPUT_SCHEMA_ID_SIZE = 2

# ── SSZ list/vector bounds ───────────────────────────────────────────────────
# Mirrors execution-specs `stateless_ssz.py` at the pinned commit (see
# requirements.txt). Re-sync if the pin moves.
#   https://github.com/ethereum/execution-specs/blob/a456712e04153ebeb17ff892446a01b6ba537f65/src/ethereum/forks/amsterdam/stateless_ssz.py
MAX_BLOB_COMMITMENTS_PER_BLOCK = 4096
MAX_WITNESS_NODES = 2**20
MAX_WITNESS_CODES = 2**16
MAX_WITNESS_HEADERS = 256
MAX_BYTES_PER_WITNESS_NODE = 2**20
MAX_BYTES_PER_CODE = 2**24
MAX_BYTES_PER_HEADER = 2**10
MAX_OPTIONAL_FORK_ACTIVATION_VALUES = 1
MAX_BLOB_SCHEDULES_PER_FORK = 1
MAX_PUBLIC_KEYS = 2**15
PUBLIC_KEY_BYTES = 65


class InvalidSsz(ValueError):
    pass


# ── SSZ wire schema (remerkleable) ───────────────────────────────────────────
#
# The execution-specs Amsterdam stateless-input schema, mirrored from
# `stateless_ssz.py` (see link above). Canonical consensus-layer leaf types
# (`cl.ExecutionPayload`, `cl.Withdrawal`, `cl.ExecutionRequests`) are copy-pasted
# verbatim in `canonical_ssz.py` and reused here.


class SszExecutionWitness(Container):
    # No `keys` field: it is not in the SSZ wire format (it is carried only in a
    # JSON/debug witness path, e.g. Zesu's). The logical `ExecutionWitness` keeps
    # it for that path; a 4-field witness here would be rejected.
    state: List[ByteList[MAX_BYTES_PER_WITNESS_NODE], MAX_WITNESS_NODES]
    codes: List[ByteList[MAX_BYTES_PER_CODE], MAX_WITNESS_CODES]
    headers: List[ByteList[MAX_BYTES_PER_HEADER], MAX_WITNESS_HEADERS]


# Amsterdam payload: canonical `ExecutionPayload` (reused from `canonical_ssz`)
# plus the two Amsterdam fields the wire carries inline — the EIP-7928 block
# access list (an opaque RLP blob, like `transactions`) and `slot_number`. Per
# `stateless_ssz.py::SszExecutionPayload`.
class SszExecutionPayload(cl.ExecutionPayload):
    block_access_list: ByteList[cl.MAX_BYTES_PER_TRANSACTION]
    slot_number: uint64


class SszNewPayloadRequest(Container):
    execution_payload: SszExecutionPayload
    versioned_hashes: List[SszBytes32, MAX_BLOB_COMMITMENTS_PER_BLOCK]
    parent_beacon_block_root: SszBytes32
    execution_requests: cl.ExecutionRequests


# ── ChainConfig (mirrored from stateless_ssz.py) ─────────────────────────────
# The fork is identified by `active_fork.fork`, the index of the active
# `ProtocolFork` in `PROTOCOL_FORKS` (see fork.py). Optional fields are modeled
# as SSZ lists of max length 1, exactly as execution-specs does.
SszOptionalForkActivationValue: TypeAlias = List[uint64, MAX_OPTIONAL_FORK_ACTIVATION_VALUES]


class SszForkActivation(Container):
    block_number: SszOptionalForkActivationValue
    timestamp: SszOptionalForkActivationValue


class SszBlobSchedule(Container):
    target: uint64
    max: uint64
    base_fee_update_fraction: uint64


SszOptionalBlobSchedule: TypeAlias = List[SszBlobSchedule, MAX_BLOB_SCHEDULES_PER_FORK]


class SszForkConfig(Container):
    fork: uint64
    activation: SszForkActivation
    blob_schedule: SszOptionalBlobSchedule


class SszChainConfig(Container):
    chain_id: uint64
    active_fork: SszForkConfig


class SszStatelessInput(Container):
    new_payload_request: SszNewPayloadRequest
    witness: SszExecutionWitness
    chain_config: SszChainConfig
    public_keys: List[ByteVector[PUBLIC_KEY_BYTES], MAX_PUBLIC_KEYS]


# ── Decode helper ────────────────────────────────────────────────────────────


def _strict_decode(data: bytes, container: type) -> Any:
    """
    Decode `data` as `container`, rejecting anything that is not its canonical
    SSZ encoding. `remerkleable.decode_bytes` is lax about length (it ignores or
    absorbs trailing bytes), so we re-encode and require equality — SSZ encoding
    is bijective.
    """
    try:
        view = container.decode_bytes(data)
    except Exception as exc:  # remerkleable raises a variety of decode errors
        raise InvalidSsz(f"{container.__name__}: {exc}") from exc
    if view.encode_bytes() != data:
        raise InvalidSsz(
            f"{container.__name__}: input is not the canonical SSZ encoding"
        )
    return view


# ── View → logical dataclass converters ──────────────────────────────────────


def _convert_withdrawal(view: Any) -> Withdrawal:
    return Withdrawal(
        index=U64(int(view.index)),
        validator_index=U64(int(view.validator_index)),
        address=Address(bytes(view.address)),
        amount=U256(int(view.amount)),
    )


def _convert_execution_payload(view: Any) -> ExecutionPayload:
    return ExecutionPayload(
        parent_hash=Hash32(bytes(view.parent_hash)),
        fee_recipient=Address(bytes(view.fee_recipient)),
        state_root=Hash32(bytes(view.state_root)),
        receipts_root=Hash32(bytes(view.receipts_root)),
        logs_bloom=bytes(view.logs_bloom),
        prev_randao=Bytes32(bytes(view.prev_randao)),
        block_number=U64(int(view.block_number)),
        gas_limit=Uint(int(view.gas_limit)),
        gas_used=Uint(int(view.gas_used)),
        timestamp=U64(int(view.timestamp)),
        extra_data=bytes(view.extra_data),
        base_fee_per_gas=Uint(int(view.base_fee_per_gas)),
        block_hash=Hash32(bytes(view.block_hash)),
        transactions=[bytes(transaction) for transaction in view.transactions],
        withdrawals=[_convert_withdrawal(withdrawal) for withdrawal in view.withdrawals],
        blob_gas_used=U64(int(view.blob_gas_used)),
        excess_blob_gas=U64(int(view.excess_blob_gas)),
        block_access_list=bytes(view.block_access_list),
        slot_number=U64(int(view.slot_number)),
    )


def _convert_deposit_request(view: Any) -> DepositRequest:
    return DepositRequest(
        pubkey=bytes(view.pubkey),
        withdrawal_credentials=Bytes32(bytes(view.withdrawal_credentials)),
        amount=U64(int(view.amount)),
        signature=bytes(view.signature),
        index=U64(int(view.index)),
    )


def _convert_withdrawal_request(view: Any) -> WithdrawalRequest:
    return WithdrawalRequest(
        source_address=Address(bytes(view.source_address)),
        validator_pubkey=bytes(view.validator_pubkey),
        amount=U64(int(view.amount)),
    )


def _convert_consolidation_request(view: Any) -> ConsolidationRequest:
    return ConsolidationRequest(
        source_address=Address(bytes(view.source_address)),
        source_pubkey=bytes(view.source_pubkey),
        target_pubkey=bytes(view.target_pubkey),
    )


def _convert_execution_requests(view: Any) -> ExecutionRequests:
    return ExecutionRequests(
        deposits=[_convert_deposit_request(deposit) for deposit in view.deposits],
        withdrawals=[
            _convert_withdrawal_request(withdrawal)
            for withdrawal in view.withdrawals
        ],
        consolidations=[
            _convert_consolidation_request(consolidation)
            for consolidation in view.consolidations
        ],
    )


def _convert_new_payload_request(view: Any) -> NewPayloadRequest:
    return NewPayloadRequest(
        execution_payload=_convert_execution_payload(view.execution_payload),
        versioned_hashes=[
            Hash32(bytes(versioned_hash)) for versioned_hash in view.versioned_hashes
        ],
        parent_beacon_block_root=Hash32(bytes(view.parent_beacon_block_root)),
        execution_requests=_convert_execution_requests(view.execution_requests),
    )


def _convert_execution_witness(view: Any) -> ExecutionWitness:
    return ExecutionWitness(
        state=[bytes(node) for node in view.state],
        codes=[bytes(code) for code in view.codes],
        headers=[bytes(header) for header in view.headers],
    )


def _convert_chain_config(view: Any) -> StatelessChainConfig:
    # `active_fork.fork` is the ProtocolFork index; reject any fork but the one
    # this spec supports (see fork.py).
    active_fork = fork.require_active_fork(int(view.active_fork.fork))
    return StatelessChainConfig(
        chain_id=U64(int(view.chain_id)),
        active_fork=active_fork,
    )


def _convert_stateless_input(view: Any) -> StatelessInput:
    return StatelessInput(
        new_payload_request=_convert_new_payload_request(view.new_payload_request),
        witness=_convert_execution_witness(view.witness),
        chain_config=_convert_chain_config(view.chain_config),
        public_keys=[bytes(public_key) for public_key in view.public_keys],
    )


def _strip_stateless_input_framing(data: bytes) -> bytes:
    """
    Strip the optional Ere length prefix and the required 0x0001 schema id,
    returning the raw `SszStatelessInput` bytes. Input without the schema id is
    rejected.
    """
    payload = bytes(data)

    # Ere wraps stdin with a 4-byte little-endian length prefix immediately
    # followed by the schema id. Strip it only when BOTH the declared length
    # matches AND the schema id appears right after: requiring the schema id
    # prevents a raw SSZ payload whose first four bytes happen to satisfy the
    # length relation from being mis-framed (which would let two distinct byte
    # strings decode to the same input, or reject a valid raw input outright).
    if (
        len(payload) >= 4 + STATELESS_INPUT_SCHEMA_ID_SIZE
        and int.from_bytes(payload[:4], "little") == len(payload) - 4
        and int.from_bytes(payload[4 : 4 + STATELESS_INPUT_SCHEMA_ID_SIZE], "big")
        == STATELESS_INPUT_SCHEMA_ID
    ):
        payload = payload[4:]

    if (
        len(payload) < STATELESS_INPUT_SCHEMA_ID_SIZE
        or int.from_bytes(payload[:STATELESS_INPUT_SCHEMA_ID_SIZE], "big")
        != STATELESS_INPUT_SCHEMA_ID
    ):
        raise InvalidSsz(
            "stateless input must begin with the 0x0001 schema id"
        )
    return payload[STATELESS_INPUT_SCHEMA_ID_SIZE:]


def decode_stateless_input_ssz(data: bytes) -> StatelessInput:
    """
    Decode the Amsterdam stateless input consumed by the guest: a 0x0001 schema
    id (optionally Ere-length-wrapped) followed by SSZ `SszStatelessInput`. The
    `active_fork` index is validated to be Amsterdam (see `fork.py`).
    """
    payload = _strip_stateless_input_framing(data)
    return _convert_stateless_input(_strict_decode(payload, SszStatelessInput))


# ── JSON object → SSZ bytes (the prover's encode step) ───────────────────────
#
# Inverse of `decode_stateless_input_ssz`. The coordinator sends the readable
# `statelessInput` object (the JSON schema form); the prover encodes it into the
# framed SSZ bytes the guest reads via `read_input`. The readable form is a
# logical view, so a few SSZ-only fields are reconstructed at canonical defaults:
#   - `chain_config.active_fork.{activation,blob_schedule}`: empty — the guest
#     validates only the fork index, taken here from `forkName`;
#   - `execution_payload.slot_number`: 0 — absent from the readable payload.


def _hexbytes(value: str) -> bytes:
    return bytes.fromhex(value[2:] if value[:2] in ("0x", "0X") else value)


def _ssz_withdrawal_from_obj(obj: dict) -> cl.Withdrawal:
    return cl.Withdrawal(
        index=int(obj["index"]),
        validator_index=int(obj["validatorIndex"]),
        address=_hexbytes(obj["address"]),
        amount=int(obj["amount"]),
    )


def _ssz_execution_payload_from_obj(obj: dict) -> SszExecutionPayload:
    return SszExecutionPayload(
        parent_hash=_hexbytes(obj["parentHash"]),
        fee_recipient=_hexbytes(obj["feeRecipient"]),
        state_root=_hexbytes(obj["stateRoot"]),
        receipts_root=_hexbytes(obj["receiptsRoot"]),
        logs_bloom=_hexbytes(obj["logsBloom"]),
        prev_randao=_hexbytes(obj["prevRandao"]),
        block_number=int(obj["blockNumber"]),
        gas_limit=int(obj["gasLimit"]),
        gas_used=int(obj["gasUsed"]),
        timestamp=int(obj["timestamp"]),
        extra_data=_hexbytes(obj["extraData"]),
        base_fee_per_gas=int(obj["baseFeePerGas"], 16),
        block_hash=_hexbytes(obj["blockHash"]),
        transactions=[_hexbytes(tx) for tx in obj["transactions"]],
        withdrawals=[_ssz_withdrawal_from_obj(w) for w in obj["withdrawals"]],
        blob_gas_used=int(obj["blobGasUsed"]),
        excess_blob_gas=int(obj["excessBlobGas"]),
        block_access_list=_hexbytes(obj["blockAccessList"]),
        slot_number=int(obj.get("slotNumber", 0)),
    )


def _ssz_execution_requests_from_obj(obj: dict) -> cl.ExecutionRequests:
    # The rollup rejects EIP-7685 requests (§2.1): all three lists must be empty.
    for key in ("deposits", "withdrawals", "consolidations"):
        if obj.get(key):
            raise InvalidSsz(f"executionRequests.{key} must be empty")
    return cl.ExecutionRequests(deposits=[], withdrawals=[], consolidations=[])


def _ssz_new_payload_request_from_obj(obj: dict) -> SszNewPayloadRequest:
    return SszNewPayloadRequest(
        execution_payload=_ssz_execution_payload_from_obj(obj["executionPayload"]),
        versioned_hashes=[_hexbytes(h) for h in obj["versionedHashes"]],
        parent_beacon_block_root=_hexbytes(obj["parentBeaconBlockRoot"]),
        execution_requests=_ssz_execution_requests_from_obj(obj["executionRequests"]),
    )


def _ssz_execution_witness_from_obj(obj: dict) -> SszExecutionWitness:
    return SszExecutionWitness(
        state=[_hexbytes(n) for n in obj["state"]],
        codes=[_hexbytes(c) for c in obj["codes"]],
        headers=[_hexbytes(h) for h in obj["headers"]],
    )


def _ssz_chain_config_from_obj(obj: dict) -> SszChainConfig:
    # The readable JSON carries only {chainId, forkName}; reconstruct the full
    # SSZ fork config. `forkName` picks the ProtocolFork (its index is the only
    # part the guest validates); activation/blob_schedule are SSZ-only, left empty.
    fork_index = fork.PROTOCOL_FORKS.index(fork.ProtocolFork(obj["forkName"]))
    fork.require_active_fork(fork_index)
    return SszChainConfig(
        chain_id=int(obj["chainId"]),
        active_fork=SszForkConfig(
            fork=fork_index,
            activation=SszForkActivation(block_number=[], timestamp=[]),
            blob_schedule=[],
        ),
    )


def _recover_public_keys(transactions: list, chain_id: int) -> list:
    """
    Recover each transaction's uncompressed SEC1 public key — the SSZ
    `public_keys` field — from the signed transactions. The readable request does
    not carry `publicKeys` (they are derivable from the transactions already in
    the payload), so the prover middleware recovers them here, mirroring how the
    guest recovers senders (`recover_sender`). `recover_transaction_public_key`
    returns the 65-byte `0x04 || x || y` key directly.
    """
    keys = []
    for tx_hex in transactions:
        tx = decode_signed_transaction_rlp(_hexbytes(tx_hex))
        keys.append(bytes(fork.recover_transaction_public_key(U64(chain_id), tx)))
    return keys


def _ssz_stateless_input_from_obj(obj: dict) -> SszStatelessInput:
    return SszStatelessInput(
        new_payload_request=_ssz_new_payload_request_from_obj(obj["newPayloadRequest"]),
        witness=_ssz_execution_witness_from_obj(obj["executionWitness"]),
        chain_config=_ssz_chain_config_from_obj(obj["chainConfig"]),
        public_keys=_recover_public_keys(
            obj["newPayloadRequest"]["executionPayload"]["transactions"],
            int(obj["chainConfig"]["chainId"]),
        ),
    )


def encode_stateless_input_ssz(obj: dict) -> bytes:
    """
    Encode a decoded `statelessInput` JSON object (schema form) into the framed
    SSZ bytes the guest reads. Inverse of `decode_stateless_input_ssz`; the
    framing is the 0x0001 schema id followed by the SSZ `SszStatelessInput`.
    """
    raw = _ssz_stateless_input_from_obj(obj).encode_bytes()
    return STATELESS_INPUT_SCHEMA_ID.to_bytes(STATELESS_INPUT_SCHEMA_ID_SIZE, "big") + raw
