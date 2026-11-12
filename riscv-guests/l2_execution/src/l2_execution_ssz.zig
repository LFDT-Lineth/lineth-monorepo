//! Wire contract of the l2-execution guest's input: the `L2ExecutionProofPrivateInput` container
//! behind the 0x0002 schema id. The decoder accepts only the canonical encoding, and the byte lists
//! alias the decoded buffer.
//!
//! Each payload's `stateless_input_ssz` is an opaque, already 0x0001-framed vanilla stateless-input
//! encoding that the execution engine decodes.

const std = @import("std");
const guest_common = @import("guest_common");
const l2_execution_proof = @import("core_serde").l2_execution_proof;

const ssz = guest_common.ssz;

pub const INPUT_SCHEMA_ID: u16 = 0x0002;

// List bounds of the wire schema: encoders and decoders reject longer lists.
pub const MAX_FTX_PER_PAYLOAD: usize = 1 << 16;
pub const MAX_STATELESS_INPUT_BYTES: usize = 1 << 30;
/// The consensus-layer Transaction byte-list bound.
pub const MAX_TX_BYTES: usize = 1 << 30;

/// System-contract addresses handed to the execution engine (field order = wire order, and matches
/// zesu's `input.SystemContracts`). Supplied by the coordinator from the node's genesis config. Not
/// part of the `dynamicChainConfigHash` preimage, which is anchored in the L1 verifier.
pub const SystemContracts = struct {
    beacon_roots: [20]u8,
    history_storage: [20]u8,
    deposit: [20]u8,
    withdrawal_request: [20]u8,
    consolidation_request: [20]u8,
    builder_deposit_request: [20]u8,
    builder_exit_request: [20]u8,

    /// Copy from any struct with the same field names (e.g. zesu's `input.SystemContracts`),
    /// without this codec depending on it.
    pub fn from(src: anytype) SystemContracts {
        var out: SystemContracts = undefined;
        inline for (std.meta.fields(SystemContracts)) |field| {
            @field(out, field.name) = @field(src, field.name);
        }
        return out;
    }
};

pub const ChainConfig = struct {
    l2_message_service_address: [20]u8,
    coinbase: [20]u8,
    chain_id: u64,
    system_contracts: SystemContracts,
};

pub const ForcedTransactionWitness = struct {
    number: u64,
    signed_tx_rlp: ssz.ByteList(MAX_TX_BYTES),
    /// The `ForcedTransactionAcceptance` enum value (0..4).
    acceptance: u8,
    deadline: u64,
};

pub const LineaPayloadInput = struct {
    stateless_input_ssz: ssz.ByteList(MAX_STATELESS_INPUT_BYTES),
    forced_transactions: ssz.VariableList(ForcedTransactionWitness, MAX_FTX_PER_PAYLOAD),
};

pub const L2ExecutionProofPrivateInput = struct {
    parent_ftx_rolling_hash: [32]u8,
    parent_last_processed_ftx_number: u64,
    chain_config: ChainConfig,
    payloads: ssz.VariableList(LineaPayloadInput, l2_execution_proof.MAX_PAYLOADS),
};

/// The public-input tuple plus the range metadata that travels outside the guest's wire output
/// (`proof` is attached by the prover layer above the guest).
pub const L2ExecutionProofOutput = struct {
    public_inputs: l2_execution_proof.L2ExecutionProofPublicInput,
    start_block_number: u64,
    tx_froms: []const [20]u8,
};

/// The input in `data`, the 0x0002 schema id followed by the SSZ container.
pub fn decodeInput(alloc: std.mem.Allocator, data: []const u8) ssz.FramedDecodeError!L2ExecutionProofPrivateInput {
    return ssz.decodeFramed(L2ExecutionProofPrivateInput, alloc, data, INPUT_SCHEMA_ID);
}

/// The frame `decodeInput` reads.
pub fn encodeInput(alloc: std.mem.Allocator, input: L2ExecutionProofPrivateInput) ssz.EncodeError![]u8 {
    return ssz.encodeFramed(L2ExecutionProofPrivateInput, alloc, INPUT_SCHEMA_ID, input);
}
