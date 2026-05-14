/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api.beacon

import com.fasterxml.jackson.annotation.JsonProperty
import com.fasterxml.jackson.databind.annotation.JsonDeserialize

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#signedbeaconblockheader
data class SignedBeaconBlockHeader(
  @JsonProperty("message") val message: BeaconBlockHeader,
  @JsonProperty("signature") val signature: String,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#beaconblockheader
@JsonDeserialize(`as` = MaruApiBeaconBlockHeader::class)
interface BeaconBlockHeader {
  val slot: String
  val proposerIndex: String
  val parentRoot: String
  val stateRoot: String
  val bodyRoot: String
}

data class MaruApiBeaconBlockHeader(
  @JsonProperty("slot") override val slot: String,
  @JsonProperty("proposer_index") override val proposerIndex: String,
  @JsonProperty("parent_root") override val parentRoot: String,
  @JsonProperty("state_root") override val stateRoot: String,
  @JsonProperty("body_root") override val bodyRoot: String,
  @JsonProperty("round") val round: String,
  @JsonProperty("timestamp") val timestamp: String,
) : BeaconBlockHeader

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#signedbeaconblock
data class SignedBeaconBlock(
  @JsonProperty("message") val message: BeaconBlock,
  @JsonProperty("signature") val signature: String,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#beaconblock
@JsonDeserialize(`as` = MaruApiSealedBeaconBlock::class)
interface BeaconBlock {
  val slot: String
  val proposerIndex: String
  val parentRoot: String
  val stateRoot: String
  val body: BeaconBlockBody
}

data class MaruApiSealedBeaconBlock(
  @JsonProperty("slot") override val slot: String,
  @JsonProperty("proposer_index") override val proposerIndex: String,
  @JsonProperty("parent_root") override val parentRoot: String,
  @JsonProperty("state_root") override val stateRoot: String,
  @JsonProperty("body") override val body: MaruApiBeaconBlockBody,
) : BeaconBlock

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/bellatrix/beacon-chain.md#beaconblockbody
@JsonDeserialize(`as` = MaruApiBeaconBlockBody::class)
interface BeaconBlockBody {
  val randaoReveal: String
  val eth1Data: Eth1Data
  val graffiti: String
  val proposerSlashings: List<ProposerSlashing>
  val attesterSlashings: List<AttesterSlashing>
  val attestations: List<Attestation>
  val deposits: List<Deposit>
  val syncAggregate: SyncAggregate
  val executionPayload: ExecutionPayload
}

data class MaruApiBeaconBlockBody(
  @JsonProperty("randao_reveal") override val randaoReveal: String,
  @JsonProperty("eth1_data") override val eth1Data: Eth1Data,
  @JsonProperty("graffiti") override val graffiti: String,
  @JsonProperty("proposer_slashings") override val proposerSlashings: List<ProposerSlashing>,
  @JsonProperty("attester_slashings") override val attesterSlashings: List<AttesterSlashing>,
  // attestations will be repurposed for commit seals of the current and previous block
  @JsonProperty("attestations") override val attestations: List<Attestation>,
  @JsonProperty("deposits") override val deposits: List<Deposit>,
  @JsonProperty("sync_aggregate") override val syncAggregate: SyncAggregate,
  @JsonProperty("execution_payload") override val executionPayload: ExecutionPayload,
) : BeaconBlockBody

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#eth1data
data class Eth1Data(
  @JsonProperty("deposit_root") val depositRoot: String,
  @JsonProperty("deposit_count") val depositCount: String,
  @JsonProperty("block_hash") val blockHash: String,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#proposerslashing
data class ProposerSlashing(
  @JsonProperty("signed_header_1") val signedHeader1: SignedBeaconBlockHeader,
  @JsonProperty("signed_header_2") val signedHeader2: SignedBeaconBlockHeader,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#attesterslashing
data class AttesterSlashing(
  @JsonProperty("attestation_1") val attestation1: IndexedAttestation,
  @JsonProperty("attestation_2") val attestation2: IndexedAttestation,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#indexedattestation
data class IndexedAttestation(
  @JsonProperty("attesting_indices") val attestingIndices: List<String> = emptyList(),
  @JsonProperty("data") val data: AttestationData,
  @JsonProperty("signature") val signature: String,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#attestationdata
data class AttestationData(
  @JsonProperty("slot") val slot: String,
  @JsonProperty("index") val index: String,
  @JsonProperty("beacon_block_root") val beaconBlockRoot: String,
  @JsonProperty("source") val source: Checkpoint,
  @JsonProperty("target") val target: Checkpoint,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#checkpoint
data class Checkpoint(
  @JsonProperty("epoch") val epoch: String,
  @JsonProperty("root") val root: String,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#attestation
data class Attestation(
  @JsonProperty("aggregation_bits") val aggregationBits: String,
  @JsonProperty("data") val data: AttestationData,
  @JsonProperty("signature") val signature: String,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#deposit
data class Deposit(
  @JsonProperty("proof") val proof: List<String>,
  @JsonProperty("data") val data: DepositData,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#depositdata
data class DepositData(
  @JsonProperty("pubkey") val pubkey: String,
  @JsonProperty("withdrawal_credentials") val withdrawalCredentials: String,
  @JsonProperty("amount") val amount: String,
  @JsonProperty("signature") val signature: String,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/altair/beacon-chain.md#syncaggregate
data class SyncAggregate(
  @JsonProperty("sync_committee_bits") val syncCommitteeBits: String,
  @JsonProperty("sync_committee_signature") val syncCommitteeSignature: String,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/bellatrix/beacon-chain.md#executionpayload
data class ExecutionPayload(
  @JsonProperty("parent_hash") val parentHash: String,
  @JsonProperty("fee_recipient") val feeRecipient: String,
  @JsonProperty("state_root") val stateRoot: String,
  @JsonProperty("receipts_root") val receiptsRoot: String,
  @JsonProperty("logs_bloom") val logsBloom: String,
  @JsonProperty("prev_randao") val prevRandao: String,
  @JsonProperty("block_number") val blockNumber: String,
  @JsonProperty("gas_limit") val gasLimit: String,
  @JsonProperty("gas_used") val gasUsed: String,
  @JsonProperty("timestamp") val timestamp: String,
  @JsonProperty("extra_data") val extraData: String,
  @JsonProperty("base_fee_per_gas") val baseFeePerGas: String,
  @JsonProperty("block_hash") val blockHash: String,
  @JsonProperty("transactions") val transactions: List<String>,
)
