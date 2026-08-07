"""
Host-side JSON <-> guest-dataclass codec for the prover proofs (V1).

This is the "shim" between the coordinator's JSON and the Python guests:

    request.json  --decode_request------------> L2ExecutionProofPrivateInput        --> run_l2_execution_guest
    response.json <--encode_response----------- L2ExecutionProof                    <--/

    request.json  --decode_rollup_request-----> RollupProofPrivateInput             --> run_rollup_guest
    response.json <--encode_rollup_response---- RollupProof                         <--/

    request.json  --decode_aggregation_request-> RollupAggregationProofPrivateInput --> run_rollup_aggregation_guest
    response.json <--encode_aggregation_response- FinalizationSubmission            <--/

It lives strictly on the prover *host* side. The guest dataclasses in
`l2_execution.py` / `rollup.py` / `block.py` stay the clean domain model and
never learn about JSON; the dependency arrow points one way only
(codec -> guest types).

Guest output vs prover output: a guest emits its public-input tuple plus the
revealed hash preimages (`l2L1Messages`, `txFroms`, `l2L1Roots`,
`filteredAddresses`). The `proof` bytes are NOT produced by the guest — the
zkVM/prover layer attaches them — so they are placeholders (`b""`) in this
reference. A response therefore equals the guest output plus `proof`; the next
proving step (or L1) consumes exactly that.

Design notes:
  - The JSON field names are NOT a clean camel->snake mapping of the dataclass
    fields; several are semantic renames. Those renames are owned here,
    explicitly, in one place.
  - Per-payload `statelessInput` is a readable JSON object (mirroring SSZ
    `StatelessInput`); the codec SSZ-encodes it into the bytes the guest reads
    via `stateless_input.encode_stateless_input_ssz` (the prover's encode step),
    and the guest decodes them with `decode_stateless_input_ssz`.
  - The JSON Schemas under `prover_io/schemas/` are the versioned wire contract:
    this codec converts schema-valid JSON to/from the guest dataclasses. The
    fixtures under `prover_io/testdata/` are the language-neutral golden vectors
    (schema-checked by the schemas' conformance test, round-trip-checked by
    `proof_io_v1_test.py`). Inline coercion (`_require`, `_bytes_from_hex`,
    `_u64`, the enum lookup) yields precise field-path errors. `proverVersion`
    (on responses) and `guestProgramId` (on requests) are routing metadata.

Conventions (Lineth): byte/hash fields are 0x-prefixed hex; integers that fit in
JSON are plain numbers but `_u64` also accepts 0x-hex strings defensively.
"""

import json
from typing import Any

from ethereum.crypto.hash import Hash32
from ethereum.state import Address
from ethereum_types.numeric import U64

from .block import (
    ChainConfig,
    ForcedTransactionAcceptance,
    ForcedTransactionWitness,
    LinethPayloadInput,
    LinethRollupExtension,
)
from .stateless_input import encode_stateless_input_ssz
from .l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPrivateInput,
    L2ExecutionProofPublicInput,
    VerifiableL2ExecutionProof,
    run_l2_execution_guest,
)
from .rollup import (
    ConflationWitness,
    RollupProof,
    RollupProofPrivateInput,
    RollupPublicInput,
    VerifiableRollupProof,
    run_rollup_guest,
)
from .l1_rollup import FinalizationSubmission
from .rollup_aggregation import (
    RollupAggregationProofPrivateInput,
    run_rollup_aggregation_guest,
)


class ProofIoError(ValueError):
    """Raised when a request/response payload does not match the V1 contract."""


# ── primitive codecs ──────────────────────────────────────────────────────────


def _require(obj: dict, key: str, ctx: str) -> Any:
    if not isinstance(obj, dict) or key not in obj:
        raise ProofIoError(f"missing required field '{ctx}{key}'")
    return obj[key]


def _require_list(obj: dict, key: str, ctx: str) -> list:
    value = _require(obj, key, ctx)
    if not isinstance(value, list):
        raise ProofIoError(f"'{ctx}{key}' must be an array, got {type(value).__name__}")
    return value


def _bytes_from_hex(value: Any, ctx: str) -> bytes:
    if not isinstance(value, str) or not value.startswith("0x"):
        raise ProofIoError(f"'{ctx}' must be a 0x-prefixed hex string, got {value!r}")
    body = value[2:]
    try:
        return bytes.fromhex(body)
    except ValueError as exc:
        raise ProofIoError(f"'{ctx}' is not valid hex: {exc}") from exc


def _u64(value: Any, ctx: str) -> U64:
    # Accept a JSON number or a 0x-hex string (Ethereum quantities show up both
    # ways across tooling); reject anything else explicitly.
    if isinstance(value, bool):  # bool is an int subclass; never a quantity
        raise ProofIoError(f"'{ctx}' must be an integer, got a boolean")
    if isinstance(value, int):
        n = value
    elif isinstance(value, str) and value.startswith("0x"):
        try:
            n = int(value, 16)
        except ValueError as exc:
            raise ProofIoError(f"'{ctx}' is not a valid hex quantity: {exc}") from exc
    else:
        raise ProofIoError(f"'{ctx}' must be an integer or 0x-hex string, got {value!r}")
    if n < 0:
        raise ProofIoError(f"'{ctx}' must be non-negative, got {n}")
    return U64(n)


def _hx(value: Any) -> str:
    """Emit a 0x-prefixed hex string from any bytes-like (empty -> '0x')."""
    return "0x" + bytes(value).hex()


# ── request: JSON dict -> guest dataclass ─────────────────────────────────────


def _decode_chain_config(obj: dict) -> ChainConfig:
    return ChainConfig(
        l2_message_service_address=Address(
            _bytes_from_hex(_require(obj, "l2MessageServiceAddress", "chainConfig."), "chainConfig.l2MessageServiceAddress")
        ),
        coinbase=Address(
            _bytes_from_hex(_require(obj, "coinbase", "chainConfig."), "chainConfig.coinbase")
        ),
        chain_id=_u64(_require(obj, "chainId", "chainConfig."), "chainConfig.chainId"),
    )


def _decode_forced_transaction(obj: dict, ctx: str) -> ForcedTransactionWitness:
    acceptance_name = _require(obj, "acceptance", ctx)
    try:
        acceptance = ForcedTransactionAcceptance[acceptance_name]
    except KeyError as exc:
        valid = ", ".join(a.name for a in ForcedTransactionAcceptance)
        raise ProofIoError(
            f"'{ctx}acceptance' must be one of [{valid}], got {acceptance_name!r}"
        ) from exc
    return ForcedTransactionWitness(
        number=_u64(_require(obj, "number", ctx), f"{ctx}number"),  # rename
        signed_tx_rlp=_bytes_from_hex(_require(obj, "signedTxRlp", ctx), f"{ctx}signedTxRlp"),
        acceptance=acceptance,
        deadline=_u64(_require(obj, "deadline", ctx), f"{ctx}deadline"),  # rename
    )


def _decode_payload(obj: dict, index: int, chain_id: int, fork_name: str) -> LinethPayloadInput:
    ctx = f"payloads[{index}]."
    stateless_input = _require(obj, "statelessInput", ctx)
    new_payload_request = _require(stateless_input, "newPayloadRequest", f"{ctx}statelessInput.")
    # EIP-7685 execution requests are a flat list on the wire (one entry per typed
    # request); the rollup rejects any (§2.1), so the list must be empty.
    execution_requests = _require(
        new_payload_request, "executionRequests", f"{ctx}statelessInput.newPayloadRequest."
    )
    if not isinstance(execution_requests, list):
        raise ProofIoError(
            f"'{ctx}statelessInput.newPayloadRequest.executionRequests' must be an array"
        )
    if execution_requests:
        raise ProofIoError(
            f"'{ctx}statelessInput.newPayloadRequest.executionRequests' must be empty (§2.1)"
        )
    # The readable `statelessInput` omits two SSZ fields the encoder needs:
    #   - `chainConfig`: carried once at `proofRequest.chainConfig`; reinject the
    #     {chainId, forkName} the SSZ `SszChainConfig` requires (chainId already
    #     coerced to int so a 0x-hex quantity survives the round-trip);
    #   - `executionRequests`: present as a flat list, but the encoder expects the
    #     deposits/withdrawals/consolidations object form — pass the empty object
    #     (we already rejected any non-empty list above).
    # The result is then SSZ-encoded into the bytes the guest reads via read_input.
    encoder_obj = {
        **stateless_input,
        "newPayloadRequest": {**new_payload_request, "executionRequests": {}},
        "chainConfig": {"chainId": chain_id, "forkName": fork_name},
    }
    try:
        stateless_input_ssz = encode_stateless_input_ssz(encoder_obj)
    except Exception as exc:  # SSZ build/encode rejects a malformed statelessInput
        raise ProofIoError(f"'{ctx}statelessInput' could not be SSZ-encoded: {exc}") from exc
    rollup_extension = _require(obj, "rollupExtension", ctx)
    forced = _require(rollup_extension, "forcedTransactions", f"{ctx}rollupExtension.")
    if not isinstance(forced, list):
        raise ProofIoError(f"'{ctx}rollupExtension.forcedTransactions' must be an array")
    return LinethPayloadInput(
        stateless_input_ssz=stateless_input_ssz,
        rollup_extension=LinethRollupExtension(
            forced_transactions=[
                _decode_forced_transaction(ftx, f"{ctx}rollupExtension.forcedTransactions[{i}].")
                for i, ftx in enumerate(forced)
            ],
        ),
    )


def decode_request(obj: dict) -> L2ExecutionProofPrivateInput:
    """
    Convert a parsed `getZkL2ExecutionProofV1.request.json` object into the guest
    input dataclass.

    The request is a `{guestProgramId, proofRequest}` envelope: `guestProgramId`
    is routing metadata and the block range is implied by the payloads. The single
    `proofRequest.chainConfig` carries both the Lineth range-level config
    (`l2MessageServiceAddress`, `coinbase`, `chainId`) and the `{chainId, forkName}`
    the per-payload stateless-input SSZ needs; `_decode_payload` reinjects the
    latter when SSZ-encoding each payload's readable `statelessInput`.
    """
    proof_request = _require(obj, "proofRequest", "")
    payloads = _require(proof_request, "payloads", "proofRequest.")
    if not isinstance(payloads, list) or not payloads:
        raise ProofIoError("'proofRequest.payloads' must be a non-empty array")
    chain_config_obj = _require(proof_request, "chainConfig", "proofRequest.")
    chain_config = _decode_chain_config(chain_config_obj)
    # `forkName` selects the stateless-input SSZ fork (the only part the guest
    # validates); it is not part of the Lineth range-level `ChainConfig` dataclass.
    fork_name = _require(chain_config_obj, "forkName", "proofRequest.chainConfig.")
    return L2ExecutionProofPrivateInput(
        parent_ftx_rolling_hash=Hash32(
            _bytes_from_hex(
                _require(proof_request, "parentFtxRollingHash", "proofRequest."),
                "proofRequest.parentFtxRollingHash",
            )
        ),
        parent_last_processed_ftx_number=_u64(
            _require(proof_request, "parentLastProcessedFtxNumber", "proofRequest."),
            "proofRequest.parentLastProcessedFtxNumber",
        ),
        chain_config=chain_config,
        payloads=[
            _decode_payload(p, i, int(chain_config.chain_id), fork_name)
            for i, p in enumerate(payloads)
        ],
    )


def decode_request_json(text: str | bytes) -> L2ExecutionProofPrivateInput:
    return decode_request(json.loads(text))


# ── response: guest dataclass -> JSON dict ────────────────────────────────────


def encode_response(proof: L2ExecutionProof, prover_version: str) -> dict:
    """
    Convert the guest's `L2ExecutionProof` into a
    `getZkL2ExecutionProofV1.response.json` object the coordinator's Jackson
    mapper consumes directly.
    """
    pi = proof.public_inputs
    return {
        "proverVersion": prover_version,
        "proof": _hx(proof.proof),
        "startBlockNumber": int(proof.start_block_number),
        "publicInputs": {
            "parentBlockHash": _hx(pi.parent_block_hash),
            "endBlockHash": _hx(pi.end_block_hash),
            "endBlockNumber": int(pi.end_block_number),
            "endBlockTimestamp": int(pi.end_block_timestamp),
            "l2L1MessagesHash": _hx(pi.l2_l1_messages_hash),
            "parentL1L2BridgeRollingHash": _hx(pi.parent_l1_l2_bridge_rolling_hash),
            "parentL1L2BridgeRollingHashMessageNumber": int(
                pi.parent_l1_l2_bridge_rolling_hash_message_number
            ),
            "endL1L2BridgeRollingHash": _hx(pi.end_l1_l2_bridge_rolling_hash),
            "endL1L2BridgeRollingHashMessageNumber": int(
                pi.end_l1_l2_bridge_rolling_hash_message_number
            ),
            "dynamicChainConfigHash": _hx(pi.dynamic_chain_config_hash),
            "parentFtxRollingHash": _hx(pi.parent_ftx_rolling_hash),
            "parentProcessedFtxNumber": int(pi.parent_processed_ftx_number),
            "endFtxRollingHash": _hx(pi.end_ftx_rolling_hash),
            "endProcessedFtxNumber": int(pi.end_processed_ftx_number),
            "filteredAddressesHash": _hx(pi.filtered_addresses_hash),
            "txFromsHash": _hx(pi.tx_froms_hash),
        },
        "l2L1Messages": [_hx(h) for h in proof.l2_l1_messages],
        "txFroms": [_hx(a) for a in proof.tx_froms],
        "filteredAddresses": [_hx(a) for a in proof.filtered_addresses],
    }


def encode_response_json(
    proof: L2ExecutionProof, prover_version: str, *, indent: int | None = None
) -> str:
    return json.dumps(encode_response(proof, prover_version), indent=indent)


# ── prover entrypoint ─────────────────────────────────────────────────────────


def run_from_request_json(text: str | bytes, prover_version: str) -> dict:
    """Full host flow: parse request JSON, run the guest, return response JSON dict."""
    execution_input = decode_request_json(text)
    proof = run_l2_execution_guest(execution_input)
    return encode_response(proof, prover_version)


# ══════════════════════════════════════════════════════════════════════════════
# Rollup proof (V1)
# ══════════════════════════════════════════════════════════════════════════════
#
# A rollup request embeds the l2-execution proofs it recursively verifies, in the
# same JSON shape the l2-execution *response* uses (minus the `proverVersion`
# envelope field). We therefore decode each embedded l2-execution proof back into
# the `L2ExecutionProof` guest dataclass — the inverse of `encode_response`.


# ── l2-execution proof: nested decode (embedded in the rollup request) ────────


def _decode_l2_execution_public_input(obj: dict, ctx: str) -> L2ExecutionProofPublicInput:
    def h(key: str) -> Hash32:
        return Hash32(_bytes_from_hex(_require(obj, key, ctx), f"{ctx}{key}"))

    def n(key: str) -> U64:
        return _u64(_require(obj, key, ctx), f"{ctx}{key}")

    return L2ExecutionProofPublicInput(
        parent_block_hash=h("parentBlockHash"),
        end_block_hash=h("endBlockHash"),
        end_block_number=n("endBlockNumber"),
        end_block_timestamp=n("endBlockTimestamp"),
        l2_l1_messages_hash=h("l2L1MessagesHash"),
        parent_l1_l2_bridge_rolling_hash=h("parentL1L2BridgeRollingHash"),
        parent_l1_l2_bridge_rolling_hash_message_number=n("parentL1L2BridgeRollingHashMessageNumber"),
        end_l1_l2_bridge_rolling_hash=h("endL1L2BridgeRollingHash"),
        end_l1_l2_bridge_rolling_hash_message_number=n("endL1L2BridgeRollingHashMessageNumber"),
        dynamic_chain_config_hash=h("dynamicChainConfigHash"),
        parent_ftx_rolling_hash=h("parentFtxRollingHash"),
        parent_processed_ftx_number=n("parentProcessedFtxNumber"),
        end_ftx_rolling_hash=h("endFtxRollingHash"),
        end_processed_ftx_number=n("endProcessedFtxNumber"),
        filtered_addresses_hash=h("filteredAddressesHash"),
        tx_froms_hash=h("txFromsHash"),
    )


def _decode_l2_execution_proof(obj: dict, ctx: str) -> VerifiableL2ExecutionProof:
    l2_l1_messages = _require_list(obj, "l2L1Messages", ctx)
    tx_froms = _require_list(obj, "txFroms", ctx)
    filtered_addresses = _require_list(obj, "filteredAddresses", ctx)
    proof = L2ExecutionProof(
        public_inputs=_decode_l2_execution_public_input(
            _require(obj, "publicInputs", ctx), f"{ctx}publicInputs."
        ),
        start_block_number=_u64(_require(obj, "startBlockNumber", ctx), f"{ctx}startBlockNumber"),
        proof=_bytes_from_hex(_require(obj, "proof", ctx), f"{ctx}proof"),
        l2_l1_messages=[
            Hash32(_bytes_from_hex(h, f"{ctx}l2L1Messages[{i}]"))
            for i, h in enumerate(l2_l1_messages)
        ],
        tx_froms=[
            Address(_bytes_from_hex(a, f"{ctx}txFroms[{i}]")) for i, a in enumerate(tx_froms)
        ],
        filtered_addresses=[
            Address(_bytes_from_hex(a, f"{ctx}filteredAddresses[{i}]"))
            for i, a in enumerate(filtered_addresses)
        ],
    )
    return VerifiableL2ExecutionProof(
        proof=proof,
        # §ProgramVK anchoring: the l2-execution proof's VK, supplied by the
        # coordinator as a runtime input for the rollup guest's recursive verify.
        program_vk=Hash32(_bytes_from_hex(_require(obj, "programVk", ctx), f"{ctx}programVk")),
    )


# ── rollup request: JSON dict -> guest dataclass ──────────────────────────────


def _decode_conflation_witness(obj: dict, ctx: str) -> ConflationWitness:
    block_rlps = _require_list(obj, "blockRlps", ctx)
    if not block_rlps:
        raise ProofIoError(f"'{ctx}blockRlps' must be a non-empty array")
    return ConflationWitness(
        block_rlps=[
            _bytes_from_hex(r, f"{ctx}blockRlps[{i}]") for i, r in enumerate(block_rlps)
        ],
    )


def decode_rollup_request(obj: dict) -> RollupProofPrivateInput:
    """
    Convert a parsed `getZkRollupProofV1.request.json` object into the rollup
    guest input dataclass.

    The request is a `{guestProgramId, proofRequest}` envelope: `guestProgramId`
    is routing metadata and the block range is implied by `conflations` (paired
    1:1 with `l2ExecutionProofs`). `chunks` is one anchored versioned hash per
    touched chunk. `opaquePrefixBytes`/`opaqueSuffixBytes`
    are top-level (not per-chunk): relevant only to the first/last touched
    chunk respectively, and empty (`0x`) when absent. `parentDrh`/`startOffset`
    are guest inputs; the outbound `endDrh`/`endOffset` are recomputed by the
    guest and returned in the response PI, so they are not echoed in the
    request. `boundaryPrevDrh` is present only for a mid-chunk start
    (`startOffset > 0`, §3.4).
    """
    proof_request = _require(obj, "proofRequest", "")
    conflations = _require_list(proof_request, "conflations", "proofRequest.")
    if not conflations:
        raise ProofIoError("'proofRequest.conflations' must be a non-empty array")
    chunks = _require_list(proof_request, "chunks", "proofRequest.")
    if not chunks:
        raise ProofIoError("'proofRequest.chunks' must be a non-empty array")
    l2_execution_proofs = _require_list(proof_request, "l2ExecutionProofs", "proofRequest.")
    if not l2_execution_proofs:
        raise ProofIoError("'proofRequest.l2ExecutionProofs' must be a non-empty array")
    if len(conflations) != len(l2_execution_proofs):
        raise ProofIoError(
            "'proofRequest.conflations' and 'proofRequest.l2ExecutionProofs' must have the same length"
        )
    start_offset = int(_u64(_require(proof_request, "startOffset", "proofRequest."), "proofRequest.startOffset"))
    boundary_prev_drh_hex = proof_request.get("boundaryPrevDrh")
    if start_offset > 0 and boundary_prev_drh_hex is None:
        raise ProofIoError("'proofRequest.boundaryPrevDrh' is required when startOffset > 0")
    opaque_prefix_bytes = _bytes_from_hex(
        proof_request.get("opaquePrefixBytes", "0x"), "proofRequest.opaquePrefixBytes"
    )
    if len(opaque_prefix_bytes) != start_offset:
        raise ProofIoError("'proofRequest.opaquePrefixBytes' length must equal startOffset")
    return RollupProofPrivateInput(
        parent_drh=Hash32(
            _bytes_from_hex(
                _require(proof_request, "parentDrh", "proofRequest."),
                "proofRequest.parentDrh",
            )
        ),
        start_offset=start_offset,
        chain_id=_u64(_require(proof_request, "chainId", "proofRequest."), "proofRequest.chainId"),
        conflations=[
            _decode_conflation_witness(c, f"proofRequest.conflations[{i}].")
            for i, c in enumerate(conflations)
        ],
        chunks=[
            Hash32(_bytes_from_hex(c, f"proofRequest.chunks[{i}]")) for i, c in enumerate(chunks)
        ],
        l2_execution_proofs=[
            _decode_l2_execution_proof(p, f"proofRequest.l2ExecutionProofs[{i}].")
            for i, p in enumerate(l2_execution_proofs)
        ],
        opaque_prefix_bytes=opaque_prefix_bytes,
        opaque_suffix_bytes=_bytes_from_hex(
            proof_request.get("opaqueSuffixBytes", "0x"), "proofRequest.opaqueSuffixBytes"
        ),
        boundary_prev_drh=(
            Hash32(_bytes_from_hex(boundary_prev_drh_hex, "proofRequest.boundaryPrevDrh"))
            if boundary_prev_drh_hex is not None
            else None
        ),
    )


def decode_rollup_request_json(text: str | bytes) -> RollupProofPrivateInput:
    return decode_rollup_request(json.loads(text))


# ── rollup response: guest dataclass -> JSON dict ─────────────────────────────


def _encode_rollup_public_inputs(pi: RollupPublicInput) -> dict:
    """The 20-field rollup PI tuple (§2.4) as JSON — shared by the rollup and
    rollup-aggregation responses, which expose the identical PI structure."""
    return {
        "endBlockNumber": int(pi.end_block_number),
        "endBlockTimestamp": int(pi.end_block_timestamp),
        "l2L1BridgeTransactionTree": _hx(pi.l2_l1_bridge_transaction_tree),
        "parentL1L2BridgeRollingHash": _hx(pi.parent_l1_l2_bridge_rolling_hash),
        "parentL1L2BridgeRollingHashMessageNumber": int(
            pi.parent_l1_l2_bridge_rolling_hash_message_number
        ),
        "endL1L2BridgeRollingHash": _hx(pi.end_l1_l2_bridge_rolling_hash),
        "endL1L2BridgeRollingHashMessageNumber": int(
            pi.end_l1_l2_bridge_rolling_hash_message_number
        ),
        "dynamicChainConfigHash": _hx(pi.dynamic_chain_config_hash),
        "parentFtxRollingHash": _hx(pi.parent_ftx_rolling_hash),
        "parentProcessedFtxNumber": int(pi.parent_processed_ftx_number),
        "endFtxRollingHash": _hx(pi.end_ftx_rolling_hash),
        "endProcessedFtxNumber": int(pi.end_processed_ftx_number),
        "filteredAddressesHash": _hx(pi.filtered_addresses_hash),
        "parentDrh": _hx(pi.parent_drh),
        "endDrh": _hx(pi.end_drh),
        "parentBlockHash": _hx(pi.parent_block_hash),
        "endBlockHash": _hx(pi.end_block_hash),
        "startOffset": int(pi.start_offset),
        "endOffset": int(pi.end_offset),
        # §ProgramVK anchoring: canonical sorted, distinct list of ALL guest
        # program VKs verified beneath this proof, checked against L1's single
        # combined approved-VK set (exec vs rollup not distinguished).
        "programVks": [_hx(v) for v in pi.program_vks],
    }


def encode_rollup_response(proof: RollupProof, prover_version: str) -> dict:
    """
    Convert the guest's `RollupProof` into a `getZkRollupProofV1.response.json`
    object the coordinator's Jackson mapper consumes directly.
    """
    return {
        "proverVersion": prover_version,
        "proof": _hx(proof.proof),
        "startBlockNumber": int(proof.start_block_number),
        "publicInputs": _encode_rollup_public_inputs(proof.public_inputs),
        "l2L1Roots": [_hx(r) for r in proof.l2_l1_roots],
        "filteredAddresses": [_hx(a) for a in proof.filtered_addresses],
    }


def encode_rollup_response_json(
    proof: RollupProof, prover_version: str, *, indent: int | None = None
) -> str:
    return json.dumps(encode_rollup_response(proof, prover_version), indent=indent)


# ── rollup prover entrypoint ──────────────────────────────────────────────────


def run_rollup_from_request_json(text: str | bytes, prover_version: str) -> dict:
    """Full host flow: parse rollup request JSON, run the guest, return response JSON dict."""
    rollup_input = decode_rollup_request_json(text)
    proof = run_rollup_guest(rollup_input)
    return encode_rollup_response(proof, prover_version)


# ══════════════════════════════════════════════════════════════════════════════
# Rollup-aggregation proof (V1)
# ══════════════════════════════════════════════════════════════════════════════
#
# A rollup-aggregation request embeds the rollup proofs it recursively verifies,
# in the same JSON shape the rollup *response* uses (minus the `proverVersion`
# envelope field). We decode each embedded rollup proof back into the
# `RollupProof` guest dataclass — the inverse of `encode_rollup_response`.


# ── rollup proof: nested decode (embedded in the aggregation request) ─────────


def _decode_rollup_public_input(obj: dict, ctx: str) -> RollupPublicInput:
    def h(key: str) -> Hash32:
        return Hash32(_bytes_from_hex(_require(obj, key, ctx), f"{ctx}{key}"))

    def n(key: str) -> U64:
        return _u64(_require(obj, key, ctx), f"{ctx}{key}")

    program_vks = _require_list(obj, "programVks", ctx)
    return RollupPublicInput(
        end_block_number=n("endBlockNumber"),
        end_block_timestamp=n("endBlockTimestamp"),
        l2_l1_bridge_transaction_tree=h("l2L1BridgeTransactionTree"),
        parent_l1_l2_bridge_rolling_hash=h("parentL1L2BridgeRollingHash"),
        parent_l1_l2_bridge_rolling_hash_message_number=n("parentL1L2BridgeRollingHashMessageNumber"),
        end_l1_l2_bridge_rolling_hash=h("endL1L2BridgeRollingHash"),
        end_l1_l2_bridge_rolling_hash_message_number=n("endL1L2BridgeRollingHashMessageNumber"),
        dynamic_chain_config_hash=h("dynamicChainConfigHash"),
        parent_ftx_rolling_hash=h("parentFtxRollingHash"),
        parent_processed_ftx_number=n("parentProcessedFtxNumber"),
        end_ftx_rolling_hash=h("endFtxRollingHash"),
        end_processed_ftx_number=n("endProcessedFtxNumber"),
        filtered_addresses_hash=h("filteredAddressesHash"),
        parent_drh=h("parentDrh"),
        end_drh=h("endDrh"),
        parent_block_hash=h("parentBlockHash"),
        end_block_hash=h("endBlockHash"),
        start_offset=int(n("startOffset")),
        end_offset=int(n("endOffset")),
        program_vks=[
            Hash32(_bytes_from_hex(v, f"{ctx}programVks[{i}]"))
            for i, v in enumerate(program_vks)
        ],
    )


def _decode_rollup_proof(obj: dict, ctx: str) -> VerifiableRollupProof:
    l2_l1_roots = _require_list(obj, "l2L1Roots", ctx)
    filtered_addresses = _require_list(obj, "filteredAddresses", ctx)
    proof = RollupProof(
        public_inputs=_decode_rollup_public_input(
            _require(obj, "publicInputs", ctx), f"{ctx}publicInputs."
        ),
        start_block_number=_u64(_require(obj, "startBlockNumber", ctx), f"{ctx}startBlockNumber"),
        proof=_bytes_from_hex(_require(obj, "proof", ctx), f"{ctx}proof"),
        l2_l1_roots=[
            Hash32(_bytes_from_hex(r, f"{ctx}l2L1Roots[{i}]")) for i, r in enumerate(l2_l1_roots)
        ],
        filtered_addresses=[
            Address(_bytes_from_hex(a, f"{ctx}filteredAddresses[{i}]"))
            for i, a in enumerate(filtered_addresses)
        ],
    )
    return VerifiableRollupProof(
        proof=proof,
        # §ProgramVK anchoring: the rollup proof's own VK, supplied by the
        # coordinator for the aggregation guest's recursive verify.
        program_vk=Hash32(_bytes_from_hex(_require(obj, "programVk", ctx), f"{ctx}programVk")),
    )


# ── rollup-aggregation request: JSON dict -> guest dataclass ──────────────────


def decode_aggregation_request(obj: dict) -> RollupAggregationProofPrivateInput:
    """
    Convert a parsed `getZkRollupAggregationProofV1.request.json` object into the
    rollup-aggregation guest input dataclass.

    The request is a `{guestProgramId, proofRequest}` envelope: `guestProgramId`
    is routing metadata and the aggregation guest input is just the flat list of
    rollup proofs. There is no `chainId` (unlike the rollup request): the
    aggregation guest does no sender recovery and inherits chain-config integrity
    from the inner proofs' `dynamicChainConfigHash`.
    """
    proof_request = _require(obj, "proofRequest", "")
    rollup_proofs = _require_list(proof_request, "rollupProofs", "proofRequest.")
    if not rollup_proofs:
        raise ProofIoError("'proofRequest.rollupProofs' must be a non-empty array")
    return RollupAggregationProofPrivateInput(
        rollup_proofs=[
            _decode_rollup_proof(p, f"proofRequest.rollupProofs[{i}].")
            for i, p in enumerate(rollup_proofs)
        ],
    )


def decode_aggregation_request_json(text: str | bytes) -> RollupAggregationProofPrivateInput:
    return decode_aggregation_request(json.loads(text))


# ── rollup-aggregation response: guest dataclass -> JSON dict ─────────────────


def encode_aggregation_response(
    submission: FinalizationSubmission,
    prover_version: str,
    *,
    start_block_number: int,
) -> dict:
    """
    Convert the rollup-aggregation guest's `FinalizationSubmission` into a
    `getZkRollupAggregationProofV1.response.json` object the coordinator's
    Jackson mapper consumes directly.

    The response equals the guest output plus `proof`, and carries the revealed
    preimages L1 needs as calldata (`l2L1Roots` for `l2L1BridgeTransactionTree`,
    `filteredAddresses` for `filteredAddressesHash`, plus `l2MessagingBlocksOffsets`),
    so it is sufficient for L1 finalization. `endBlockNumber` lives in
    `publicInputs`; only `startBlockNumber` (not in the PI tuple) is supplied as
    host-side range metadata.
    """
    return {
        "proverVersion": prover_version,
        "proof": _hx(submission.proof),
        "startBlockNumber": int(start_block_number),
        "publicInputs": _encode_rollup_public_inputs(submission.public_inputs),
        "l2L1Roots": [_hx(r) for r in submission.l2_l1_roots],
        "filteredAddresses": [_hx(a) for a in submission.filtered_addresses],
        "l2MessagingBlocksOffsets": list(submission.l2_messaging_blocks_offsets),
    }


def encode_aggregation_response_json(
    submission: FinalizationSubmission,
    prover_version: str,
    *,
    start_block_number: int,
    indent: int | None = None,
) -> str:
    return json.dumps(
        encode_aggregation_response(
            submission, prover_version, start_block_number=start_block_number
        ),
        indent=indent,
    )


# ── rollup-aggregation prover entrypoint ──────────────────────────────────────


def run_aggregation_from_request_json(text: str | bytes, prover_version: str) -> dict:
    """Full host flow: parse aggregation request JSON, run the guest, return response JSON dict."""
    aggregation_input = decode_aggregation_request_json(text)
    submission = run_rollup_aggregation_guest(aggregation_input)
    # startBlockNumber is not part of the PI tuple; take it from the first
    # rollup proof's range (the aggregation covers a contiguous range).
    start_block_number = int(aggregation_input.rollup_proofs[0].start_block_number)
    return encode_aggregation_response(
        submission, prover_version, start_block_number=start_block_number
    )
