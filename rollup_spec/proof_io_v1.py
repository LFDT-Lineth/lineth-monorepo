"""
Host-side JSON <-> guest-dataclass codec for the l2-execution proof (V1).

This is the "shim" between the coordinator's JSON and the Python guest:

    request.json  --decode_request-->  L2ExecutionProofPrivateInput  --> run_l2_execution_guest
    response.json <--encode_response-- L2ExecutionProof              <--/

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

Conventions (Linea): byte/hash fields are 0x-prefixed hex; integers that fit in
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
    LineaPayloadInput,
    LineaRollupExtension,
)
from .l2_execution import (
    L2ExecutionProof,
    L2ExecutionProofPrivateInput,
    run_l2_execution_guest,
)


class ProofIoError(ValueError):
    """Raised when a request/response payload does not match the V1 contract."""


# ── primitive codecs ──────────────────────────────────────────────────────────


def _require(obj: dict, key: str, ctx: str) -> Any:
    if not isinstance(obj, dict) or key not in obj:
        raise ProofIoError(f"missing required field '{ctx}{key}'")
    return obj[key]


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


def run_from_request_json(text: str | bytes, prover_version: str) -> dict:
    """Full host flow: parse request JSON, run the guest, return response JSON dict."""
    execution_input = decode_request_json(text)
    proof = run_l2_execution_guest(execution_input)
    return encode_response(proof, prover_version)
