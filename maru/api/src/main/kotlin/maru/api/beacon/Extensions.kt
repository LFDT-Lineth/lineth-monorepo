/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api.beacon

import linea.kotlin.encodeHex
import maru.core.BeaconBlockHeader as MaruBeaconBlockHeader
import maru.core.ExecutionPayload as MaruExecutionPayload
import maru.core.Seal as MaruSeal
import maru.core.SealedBeaconBlock as MaruSealedBeaconBlock

fun MaruSealedBeaconBlock.toBeaconBlock(): BeaconBlock {
  val maruHeader = this.beaconBlock.beaconBlockHeader
  val currentBlockAttestations =
    this.commitSeals.map {
      it.toAttestation(
        blockNumber = maruHeader.number,
        beaconBlockRoot = maruHeader.hash(),
      )
    }

  val maruBody = this.beaconBlock.beaconBlockBody
  val prevBlockAttestations =
    if (maruHeader.number > 0u) {
      maruBody.prevCommitSeals.map {
        it.toAttestation(
          blockNumber = maruHeader.number - 1u,
          beaconBlockRoot = maruHeader.parentRoot,
        )
      }
    } else {
      emptyList()
    }

  val blockBody =
    MaruApiBeaconBlockBody(
      randaoReveal = "0x",
      eth1Data = Eth1Data(
        depositCount = "0",
        depositRoot = "0x",
        blockHash = "0x",
      ),
      graffiti = "0x",
      proposerSlashings = emptyList(),
      attesterSlashings = emptyList(),
      attestations = currentBlockAttestations + prevBlockAttestations,
      deposits = emptyList(),
      syncAggregate = SyncAggregate(
        syncCommitteeBits = "0x",
        syncCommitteeSignature = "0x",
      ),
      executionPayload = maruBody.executionPayload.toExecutionPayload(),
    )

  val header = maruHeader.toBeaconBlockHeader()
  return MaruApiSealedBeaconBlock(
    slot = header.slot,
    proposerIndex = header.proposerIndex,
    parentRoot = header.parentRoot,
    stateRoot = header.stateRoot,
    body = blockBody,
  )
}

fun MaruBeaconBlockHeader.toBeaconBlockHeader(): BeaconBlockHeader =
  MaruApiBeaconBlockHeader(
    slot = this.number.toString(),
    round = this.round.toString(),
    timestamp = this.timestamp.toString(),
    proposerIndex = this.proposer.address.encodeHex(),
    parentRoot = this.parentRoot.encodeHex(),
    stateRoot = this.stateRoot.encodeHex(),
    bodyRoot = this.bodyRoot.encodeHex(),
  )

fun MaruSeal.toAttestation(
  blockNumber: ULong,
  beaconBlockRoot: ByteArray,
): Attestation {
  val attestationData =
    AttestationData(
      slot = blockNumber.toString(),
      index = "0",
      beaconBlockRoot = beaconBlockRoot.encodeHex(),
      source = Checkpoint("0", "0x"),
      target = Checkpoint("0", "0x"),
    )
  return Attestation(
    aggregationBits = "0x",
    data = attestationData,
    signature = this.signature.encodeHex(),
  )
}

private fun MaruExecutionPayload.toExecutionPayload(): ExecutionPayload =
  ExecutionPayload(
    parentHash = this.parentHash.encodeHex(),
    feeRecipient = this.feeRecipient.encodeHex(),
    stateRoot = this.stateRoot.encodeHex(),
    receiptsRoot = this.receiptsRoot.encodeHex(),
    logsBloom = this.logsBloom.encodeHex(),
    prevRandao = this.prevRandao.encodeHex(),
    blockNumber = this.blockNumber.toString(),
    gasLimit = this.gasLimit.toString(),
    gasUsed = this.gasUsed.toString(),
    timestamp = this.timestamp.toString(),
    extraData = this.extraData.encodeHex(),
    baseFeePerGas = this.baseFeePerGas.toString(),
    blockHash = this.blockHash.encodeHex(),
    transactions = this.transactions.map { it.encodeHex() },
  )
