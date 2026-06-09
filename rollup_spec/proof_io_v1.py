"""
Host-side JSON <-> guest-dataclass codec for the l2-execution proof (V1).

This is the "shim" between the coordinator's JSON and the Python guests:

    request.json  --decode_request-------->  L2ExecutionProofPrivateInput  --> run_l2_execution_guest
    response.json <--encode_response-------- L2ExecutionProof              <--/

    request.json  --decode_rollup_request-->  RollupProofPrivateInput      --> run_rollup_guest
    response.json <--encode_rollup_response-- RollupProof                  <--/

It lives strictly on the prover *host* side. The guest dataclasses in
`l2_execution.py` / `block.py` stay the clean domain model and never learn about
JSON; the dependency arrow points one way only (codec -> guest types).

Design notes:
  - The JSON field names are NOT a clean camel->snake mapping of the dataclass
    fields; several are semantic renames. Those renames are owned here, explicitly, in one place.
  - Per-payload `statelessInputSsz` stays opaque bytes here; the guest path
    decodes it on its own via `stateless_input.py::decode_stateless_input_ssz`.
    The `_debugStatelessInput` mirror in the request is review-only and is
    intentionally discarded (see §3.3 of the spec Readme).
  - The wire contract is pinned by the JSON Schemas under `rollup_spec/schemas/`;
    only `proverVersion` (a version tag) travels on the wire, never the schema.
    Inline coercion (`_require`, `_bytes_from_hex`, `_u64`, the enum lookup) is the
    primary validation and yields precise field-path errors. The `*_json` and
    `run_*_from_request_json` entry points additionally accept `validate=True` to
    enforce the JSON Schema (e.g. rejecting unknown/extra fields) at the trust
    boundary; `jsonschema` is imported lazily so it stays an optional dependency.

Conventions (Linea): byte/hash fields are 0x-prefixed hex; integers that fit in
JSON are plain numbers but `_u64` also accepts 0x-hex strings defensively.
"""

import json
from pathlib import Path
from typing import Any

from ethereum.crypto.hash import Hash32
from ethereum.state import Address
from ethereum_types.bytes import Bytes48
from ethereum_types.numeric import U64

from .block import (
    ChainConfig,
    ForcedTransactionAcceptance,
    ForcedTransactionWitness,
    LineaPayloadInput,
    LineaRollupExtension,
)
from .l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPrivateInput,
    L2ExecutionProofPublicInput,
    run_l2_execution_guest,
)
from .rollup import (
    BlobWitness,
    RollupProof,
    RollupProofPrivateInput,
    RollupPublicInput,
    run_rollup_guest,
)
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


# ── optional JSON Schema validation ───────────────────────────────────────────
#
# Schema files live next to the code so they ship with the package and stay the
# single cross-language contract artifact. Validation is opt-in (`validate=True`)
# and `jsonschema` is imported lazily, so callers that only need the inline
# coercion incur neither the dependency nor the cost.

_SCHEMA_DIR = Path(__file__).parent / "schemas"

_L2_REQUEST_SCHEMA = "getZkL2ExecutionProofV1.request.schema.json"
_L2_RESPONSE_SCHEMA = "getZkL2ExecutionProofV1.response.schema.json"
_ROLLUP_REQUEST_SCHEMA = "getZkRollupProofV1.request.schema.json"
_ROLLUP_RESPONSE_SCHEMA = "getZkRollupProofV1.response.schema.json"
_AGGREGATION_REQUEST_SCHEMA = "getZkRollupAggregationProofV1.request.schema.json"
_AGGREGATION_RESPONSE_SCHEMA = "getZkRollupAggregationProofV1.response.schema.json"


def _validate_against_schema(obj: Any, schema_filename: str) -> Any:
    """
    Validate `obj` against the named JSON Schema under `rollup_spec/schemas/` and
    return it unchanged. Raises `ProofIoError` on a schema violation — or if the
    optional `jsonschema` dependency is not installed.

    This is the only check that catches *unknown/extra* fields (the schemas set
    `additionalProperties: false`), which the inline coercion intentionally
    ignores.
    """
    try:
        import jsonschema
    except ImportError as exc:  # pragma: no cover - exercised only without the extra
        raise ProofIoError(
            "schema validation requested (validate=True) but the optional "
            "'jsonschema' dependency is not installed; install it (see "
            "rollup_spec/requirements.txt) or call with validate=False"
        ) from exc

    schema = json.loads((_SCHEMA_DIR / schema_filename).read_text())
    try:
        jsonschema.validate(obj, schema, cls=jsonschema.Draft202012Validator)
    except jsonschema.ValidationError as exc:
        raise ProofIoError(
            f"payload does not conform to {schema_filename} "
            f"at {exc.json_path}: {exc.message}"
        ) from exc
    return obj


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


def _decode_payload(obj: dict, index: int) -> LineaPayloadInput:
    ctx = f"payloads[{index}]."
    # `_debugStatelessInput` is review-only and deliberately ignored here.
    rollup_extension = _require(obj, "rollupExtension", ctx)
    forced = _require(rollup_extension, "forcedTransactions", f"{ctx}rollupExtension.")
    if not isinstance(forced, list):
        raise ProofIoError(f"'{ctx}rollupExtension.forcedTransactions' must be an array")
    return LineaPayloadInput(
        stateless_input_ssz=_bytes_from_hex(
            _require(obj, "statelessInputSsz", ctx), f"{ctx}statelessInputSsz"
        ),
        rollup_extension=LineaRollupExtension(
            forced_transactions=[
                _decode_forced_transaction(ftx, f"{ctx}rollupExtension.forcedTransactions[{i}].")
                for i, ftx in enumerate(forced)
            ],
        ),
    )


def decode_request(obj: dict) -> L2ExecutionProofPrivateInput:
    """
    Convert a parsed `getZkL2ExecutionProofV1.request.json` object into the guest
    input dataclass. `proverVersion` / `blockRange` are request metadata and are
    not part of the guest input (block range is implied by the payloads).
    """
    payloads = _require(obj, "payloads", "")
    if not isinstance(payloads, list) or not payloads:
        raise ProofIoError("'payloads' must be a non-empty array")
    return L2ExecutionProofPrivateInput(
        parent_ftx_rolling_hash=Hash32(
            _bytes_from_hex(_require(obj, "parentFtxRollingHash", ""), "parentFtxRollingHash")
        ),
        parent_last_processed_ftx_number=_u64(
            _require(obj, "parentLastProcessedFtxNumber", ""), "parentLastProcessedFtxNumber"
        ),
        chain_config=_decode_chain_config(_require(obj, "chainConfig", "")),
        payloads=[_decode_payload(p, i) for i, p in enumerate(payloads)],
    )


def decode_request_json(
    text: str | bytes, *, validate: bool = False
) -> L2ExecutionProofPrivateInput:
    obj = json.loads(text)
    if validate:
        _validate_against_schema(obj, _L2_REQUEST_SCHEMA)
    return decode_request(obj)


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
        "endBlockNumber": int(proof.end_block_number),
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
            "endFtxRollingHash": _hx(pi.end_ftx_rolling_hash),
            "lastProcessedFtxNumber": int(pi.last_processed_ftx_number),
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


def run_from_request_json(
    text: str | bytes, prover_version: str, *, validate: bool = False
) -> dict:
    """
    Full host flow: parse request JSON, run the guest, return response JSON dict.

    With `validate=True`, the incoming request and the emitted response are both
    checked against their JSON Schemas (defense-in-depth at the trust boundary).
    """
    execution_input = decode_request_json(text, validate=validate)
    proof = run_l2_execution_guest(execution_input)
    response = encode_response(proof, prover_version)
    if validate:
        _validate_against_schema(response, _L2_RESPONSE_SCHEMA)
    return response


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
        end_ftx_rolling_hash=h("endFtxRollingHash"),
        last_processed_ftx_number=n("lastProcessedFtxNumber"),
        filtered_addresses_hash=h("filteredAddressesHash"),
        tx_froms_hash=h("txFromsHash"),
    )


def _decode_l2_execution_proof(obj: dict, ctx: str) -> L2ExecutionProof:
    l2_l1_messages = _require_list(obj, "l2L1Messages", ctx)
    tx_froms = _require_list(obj, "txFroms", ctx)
    filtered_addresses = _require_list(obj, "filteredAddresses", ctx)
    return L2ExecutionProof(
        public_inputs=_decode_l2_execution_public_input(
            _require(obj, "publicInputs", ctx), f"{ctx}publicInputs."
        ),
        start_block_number=_u64(_require(obj, "startBlockNumber", ctx), f"{ctx}startBlockNumber"),
        end_block_number=_u64(_require(obj, "endBlockNumber", ctx), f"{ctx}endBlockNumber"),
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


# ── rollup request: JSON dict -> guest dataclass ──────────────────────────────


def _decode_blob_witness(obj: dict, ctx: str) -> BlobWitness:
    blob_inputs = _require(obj, "blobInputs", ctx)
    block_range = _require(obj, "blockRange", ctx)
    start = _u64(_require(block_range, "startBlockNumber", f"{ctx}blockRange."), f"{ctx}blockRange.startBlockNumber")
    end = _u64(_require(block_range, "endBlockNumber", f"{ctx}blockRange."), f"{ctx}blockRange.endBlockNumber")
    block_rlps = _require_list(obj, "blockRlps", ctx)
    return BlobWitness(
        block_number_range=(int(start), int(end)),
        block_rlps=[
            _bytes_from_hex(r, f"{ctx}blockRlps[{i}]") for i, r in enumerate(block_rlps)
        ],
        blob_hash=Hash32(
            _bytes_from_hex(_require(blob_inputs, "blobHash", f"{ctx}blobInputs."), f"{ctx}blobInputs.blobHash")
        ),
        blob_kzg_proof=Bytes48(
            _bytes_from_hex(
                _require(blob_inputs, "blobKzgProof", f"{ctx}blobInputs."), f"{ctx}blobInputs.blobKzgProof"
            )
        ),
    )


def decode_rollup_request(obj: dict) -> RollupProofPrivateInput:
    """
    Convert a parsed `getZkRollupProofV1.request.json` object into the rollup
    guest input dataclass.

    `proverVersion` and the top-level `blockRange` are request metadata (the
    block range is implied by the blobs). `shnarfTransition.endShnarf` is the
    *expected* output the guest recomputes and asserts — it is not part of the
    private input, so only `parentShnarf` is carried into the dataclass.
    """
    blobs = _require_list(obj, "blobs", "")
    if not blobs:
        raise ProofIoError("'blobs' must be a non-empty array")
    l2_execution_proofs = _require_list(obj, "l2ExecutionProofs", "")
    if not l2_execution_proofs:
        raise ProofIoError("'l2ExecutionProofs' must be a non-empty array")
    shnarf_transition = _require(obj, "shnarfTransition", "")
    return RollupProofPrivateInput(
        parent_shnarf=Hash32(
            _bytes_from_hex(
                _require(shnarf_transition, "parentShnarf", "shnarfTransition."),
                "shnarfTransition.parentShnarf",
            )
        ),
        chain_id=_u64(_require(obj, "chainId", ""), "chainId"),
        blobs=[_decode_blob_witness(b, f"blobs[{i}].") for i, b in enumerate(blobs)],
        l2_execution_proofs=[
            _decode_l2_execution_proof(p, f"l2ExecutionProofs[{i}].")
            for i, p in enumerate(l2_execution_proofs)
        ],
    )


def decode_rollup_request_json(
    text: str | bytes, *, validate: bool = False
) -> RollupProofPrivateInput:
    obj = json.loads(text)
    if validate:
        _validate_against_schema(obj, _ROLLUP_REQUEST_SCHEMA)
    return decode_rollup_request(obj)


# ── rollup response: guest dataclass -> JSON dict ─────────────────────────────


def _encode_rollup_public_inputs(pi: RollupPublicInput) -> dict:
    """The 14-field rollup PI tuple (§2.4) as JSON — shared by the rollup and
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
        "endFtxRollingHash": _hx(pi.end_ftx_rolling_hash),
        "lastProcessedFtxNumber": int(pi.last_processed_ftx_number),
        "filteredAddressesHash": _hx(pi.filtered_addresses_hash),
        "parentShnarf": _hx(pi.parent_shnarf),
        "endShnarf": _hx(pi.end_shnarf),
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
        "endBlockNumber": int(proof.end_block_number),
        "publicInputs": _encode_rollup_public_inputs(proof.public_inputs),
        "l2L1Roots": [_hx(r) for r in proof.l2_l1_roots],
        "filteredAddresses": [_hx(a) for a in proof.filtered_addresses],
    }


def encode_rollup_response_json(
    proof: RollupProof, prover_version: str, *, indent: int | None = None
) -> str:
    return json.dumps(encode_rollup_response(proof, prover_version), indent=indent)


# ── rollup prover entrypoint ──────────────────────────────────────────────────


def run_rollup_from_request_json(
    text: str | bytes, prover_version: str, *, validate: bool = False
) -> dict:
    """
    Full host flow: parse rollup request JSON, run the guest, return response JSON dict.

    With `validate=True`, the incoming request and the emitted response are both
    checked against their JSON Schemas.
    """
    rollup_input = decode_rollup_request_json(text, validate=validate)
    proof = run_rollup_guest(rollup_input)
    response = encode_rollup_response(proof, prover_version)
    if validate:
        _validate_against_schema(response, _ROLLUP_RESPONSE_SCHEMA)
    return response


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
        end_ftx_rolling_hash=h("endFtxRollingHash"),
        last_processed_ftx_number=n("lastProcessedFtxNumber"),
        filtered_addresses_hash=h("filteredAddressesHash"),
        parent_shnarf=h("parentShnarf"),
        end_shnarf=h("endShnarf"),
    )


def _decode_rollup_proof(obj: dict, ctx: str) -> RollupProof:
    l2_l1_roots = _require_list(obj, "l2L1Roots", ctx)
    filtered_addresses = _require_list(obj, "filteredAddresses", ctx)
    return RollupProof(
        public_inputs=_decode_rollup_public_input(
            _require(obj, "publicInputs", ctx), f"{ctx}publicInputs."
        ),
        start_block_number=_u64(_require(obj, "startBlockNumber", ctx), f"{ctx}startBlockNumber"),
        end_block_number=_u64(_require(obj, "endBlockNumber", ctx), f"{ctx}endBlockNumber"),
        proof=_bytes_from_hex(_require(obj, "proof", ctx), f"{ctx}proof"),
        l2_l1_roots=[
            Hash32(_bytes_from_hex(r, f"{ctx}l2L1Roots[{i}]")) for i, r in enumerate(l2_l1_roots)
        ],
        filtered_addresses=[
            Address(_bytes_from_hex(a, f"{ctx}filteredAddresses[{i}]"))
            for i, a in enumerate(filtered_addresses)
        ],
    )


# ── rollup-aggregation request: JSON dict -> guest dataclass ──────────────────


def decode_aggregation_request(obj: dict) -> RollupAggregationProofPrivateInput:
    """
    Convert a parsed `getZkRollupAggregationProofV1.request.json` object into the
    rollup-aggregation guest input dataclass.

    `proverVersion`, `chainId`, and the top-level `blockRange` are request
    metadata; the aggregation guest input is just the flat list of rollup proofs.
    """
    rollup_proofs = _require_list(obj, "rollupProofs", "")
    if not rollup_proofs:
        raise ProofIoError("'rollupProofs' must be a non-empty array")
    return RollupAggregationProofPrivateInput(
        rollup_proofs=[
            _decode_rollup_proof(p, f"rollupProofs[{i}].") for i, p in enumerate(rollup_proofs)
        ],
    )


def decode_aggregation_request_json(
    text: str | bytes, *, validate: bool = False
) -> RollupAggregationProofPrivateInput:
    obj = json.loads(text)
    if validate:
        _validate_against_schema(obj, _AGGREGATION_REQUEST_SCHEMA)
    return decode_aggregation_request(obj)


# ── rollup-aggregation response: guest dataclass -> JSON dict ─────────────────


def encode_aggregation_response(
    public_inputs: RollupPublicInput,
    prover_version: str,
    *,
    start_block_number: int,
    proof: bytes = b"",
) -> dict:
    """
    Convert the rollup-aggregation guest's `RollupPublicInput` into a
    `getZkRollupAggregationProofV1.response.json` object.

    Unlike the rollup proof, the aggregation guest returns only the public-input
    tuple (`run_rollup_aggregation_guest` -> `RollupPublicInput`). The SNARK
    `proof` bytes and the range's `startBlockNumber` are host-side metadata
    supplied here: `endBlockNumber` comes from the PI, but `startBlockNumber` is
    not part of the PI tuple (the rollup PI does not expose it).
    """
    return {
        "proverVersion": prover_version,
        "proof": _hx(proof),
        "startBlockNumber": int(start_block_number),
        "endBlockNumber": int(public_inputs.end_block_number),
        "publicInputs": _encode_rollup_public_inputs(public_inputs),
    }


def encode_aggregation_response_json(
    public_inputs: RollupPublicInput,
    prover_version: str,
    *,
    start_block_number: int,
    proof: bytes = b"",
    indent: int | None = None,
) -> str:
    return json.dumps(
        encode_aggregation_response(
            public_inputs, prover_version, start_block_number=start_block_number, proof=proof
        ),
        indent=indent,
    )


# ── rollup-aggregation prover entrypoint ──────────────────────────────────────


def run_aggregation_from_request_json(
    text: str | bytes, prover_version: str, *, validate: bool = False
) -> dict:
    """
    Full host flow: parse aggregation request JSON, run the guest, return response JSON dict.

    With `validate=True`, the incoming request and the emitted response are both
    checked against their JSON Schemas.
    """
    aggregation_input = decode_aggregation_request_json(text, validate=validate)
    public_inputs = run_rollup_aggregation_guest(aggregation_input)
    # startBlockNumber is not part of the PI tuple; take it from the first
    # rollup proof's range (the aggregation covers a contiguous range).
    start_block_number = int(aggregation_input.rollup_proofs[0].start_block_number)
    response = encode_aggregation_response(
        public_inputs, prover_version, start_block_number=start_block_number
    )
    if validate:
        _validate_against_schema(response, _AGGREGATION_RESPONSE_SCHEMA)
    return response
