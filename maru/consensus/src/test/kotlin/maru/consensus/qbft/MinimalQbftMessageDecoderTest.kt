/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.consensus.qbft.adapters.QbftBlockAdapter
import maru.consensus.qbft.adapters.QbftBlockCodecAdapter
import maru.core.ext.DataGenerators
import maru.crypto.PrivateKeyGenerator
import maru.crypto.SecpCrypto
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.payload.SignedData
import org.hyperledger.besu.consensus.qbft.core.messagedata.CommitMessageData
import org.hyperledger.besu.consensus.qbft.core.messagedata.PrepareMessageData
import org.hyperledger.besu.consensus.qbft.core.messagedata.ProposalMessageData
import org.hyperledger.besu.consensus.qbft.core.messagedata.QbftV1
import org.hyperledger.besu.consensus.qbft.core.messagedata.RoundChangeMessageData
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Commit
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Prepare
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Proposal
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.RoundChange
import org.hyperledger.besu.consensus.qbft.core.payload.CommitPayload
import org.hyperledger.besu.consensus.qbft.core.payload.PreparePayload
import org.hyperledger.besu.consensus.qbft.core.payload.ProposalPayload
import org.hyperledger.besu.consensus.qbft.core.payload.RoundChangePayload
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Hash
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import java.util.Optional
import kotlin.random.Random

class MinimalQbftMessageDecoderTest {
  private val keyData = PrivateKeyGenerator.generatePrivateKey()
  private val nodeKey = keyData.nodeKey
  private val messageAuthor = Address.wrap(Bytes.wrap(keyData.address))
  private val decoder = MinimalQbftMessageDecoder(SecpCrypto)

  private val sequenceNumber = 100
  private val roundNumber = 15
  private val roundIdentifier = ConsensusRoundIdentifier(sequenceNumber.toLong(), roundNumber)
  private val blockHash = Hash.hash(Bytes.wrap(Random.nextBytes(32)))

  @Test
  fun `should decode Prepare message`() {
    val preparePayload = PreparePayload(roundIdentifier, blockHash)
    val signature = nodeKey.sign(Bytes32.wrap(preparePayload.hashForSignature().bytes))
    val signedPayload = SignedData.create(preparePayload, signature)
    val prepare = Prepare(signedPayload)
    val messageData = PrepareMessageData.create(prepare)
    val qbftMessage = mock<QbftMessage>()
    whenever(qbftMessage.data).thenReturn(messageData)

    val metadata = decoder.deserialize(qbftMessage)
    assertThat(metadata.messageCode).isEqualTo(QbftV1.PREPARE)
    assertThat(metadata.sequenceNumber).isEqualTo(sequenceNumber.toLong())
    assertThat(metadata.roundNumber).isEqualTo(roundNumber.toLong())
    assertThat(metadata.author).isEqualTo(messageAuthor)
  }

  @Test
  fun `should decode Commit message`() {
    val commitSeal = nodeKey.sign(Bytes32.wrap(blockHash.bytes))
    val commitPayload = CommitPayload(roundIdentifier, blockHash, commitSeal)
    val signature = nodeKey.sign(Bytes32.wrap(commitPayload.hashForSignature().bytes))
    val signedPayload = SignedData.create(commitPayload, signature)
    val commit = Commit(signedPayload)
    val messageData = CommitMessageData.create(commit)
    val qbftMessage = mock<QbftMessage>()
    whenever(qbftMessage.data).thenReturn(messageData)

    val metadata = decoder.deserialize(qbftMessage)
    assertThat(metadata.messageCode).isEqualTo(QbftV1.COMMIT)
    assertThat(metadata.sequenceNumber).isEqualTo(sequenceNumber.toLong())
    assertThat(metadata.roundNumber).isEqualTo(roundNumber.toLong())
    assertThat(metadata.author).isEqualTo(messageAuthor)
  }

  @Test
  fun `should decode Proposal message`() {
    val beaconBlock =
      DataGenerators.randomBeaconBlock(
        sequenceNumber.toULong(),
      )
    val qbftBlock = QbftBlockAdapter(beaconBlock)
    val proposalPayload = ProposalPayload(roundIdentifier, qbftBlock, QbftBlockCodecAdapter)
    val signature = nodeKey.sign(Bytes32.wrap(proposalPayload.hashForSignature().bytes))
    val signedPayload = SignedData.create(proposalPayload, signature)
    val proposal = Proposal(signedPayload, emptyList(), emptyList())
    val messageData = ProposalMessageData.create(proposal)
    val qbftMessage = mock<QbftMessage>()
    whenever(qbftMessage.data).thenReturn(messageData)

    val metadata = decoder.deserialize(qbftMessage)
    assertThat(metadata.messageCode).isEqualTo(QbftV1.PROPOSAL)
    assertThat(metadata.sequenceNumber).isEqualTo(sequenceNumber.toLong())
    assertThat(metadata.roundNumber).isEqualTo(roundNumber.toLong())
    assertThat(metadata.author).isEqualTo(messageAuthor)
  }

  @Test
  fun `should decode RoundChange message`() {
    val roundChangePayload = RoundChangePayload(roundIdentifier, Optional.empty())
    val signature = nodeKey.sign(Bytes32.wrap(roundChangePayload.hashForSignature().bytes))
    val signedPayload = SignedData.create(roundChangePayload, signature)
    val roundChange = RoundChange(signedPayload, Optional.empty(), Optional.empty(), QbftBlockCodecAdapter, emptyList())
    val messageData = RoundChangeMessageData.create(roundChange)
    val qbftMessage = mock<QbftMessage>()
    whenever(qbftMessage.data).thenReturn(messageData)

    val metadata = decoder.deserialize(qbftMessage)
    assertThat(metadata.messageCode).isEqualTo(QbftV1.ROUND_CHANGE)
    assertThat(metadata.sequenceNumber).isEqualTo(sequenceNumber.toLong())
    assertThat(metadata.roundNumber).isEqualTo(roundNumber.toLong())
    assertThat(metadata.author).isEqualTo(messageAuthor)
  }
}
