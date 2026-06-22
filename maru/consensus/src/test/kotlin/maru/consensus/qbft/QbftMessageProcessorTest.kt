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
import maru.consensus.qbft.adapters.QbftBlockchainAdapter
import maru.consensus.qbft.adapters.QbftValidatorProviderAdapter
import maru.core.SealedBeaconBlock
import maru.core.ext.DataGenerators
import maru.crypto.PrivateKeyGenerator
import maru.crypto.SecpCrypto
import maru.database.InMemoryBeaconChain
import maru.p2p.ValidationResult.Companion.Ignore
import maru.p2p.ValidationResultCode
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes.EMPTY
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.consensus.common.bft.BftEventQueue
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.payload.SignedData
import org.hyperledger.besu.consensus.qbft.core.messagedata.ProposalMessageData
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Proposal
import org.hyperledger.besu.consensus.qbft.core.payload.ProposalPayload
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import org.hyperledger.besu.consensus.qbft.core.types.QbftReceivedMessageEvent
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.ethereum.p2p.rlpx.wire.MessageData
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import java.util.concurrent.TimeUnit

class QbftMessageProcessorTest {
  private val keyData = PrivateKeyGenerator.generatePrivateKey()
  private val nodeKey = keyData.nodeKey
  private val messageAuthor = Address.wrap(Bytes.wrap(keyData.address))
  private val localAddress = Address.fromHexString("0x1234567890123456789012345678901234567890")

  private val validatorProvider = mock<QbftValidatorProviderAdapter>()
  private val bftEventQueue = BftEventQueue(10)
  private val messageDecoder = MinimalQbftMessageDecoder(SecpCrypto)

  init {
    bftEventQueue.start()
  }

  private fun createMessageProcessor(chainHeight: ULong): QbftMessageProcessor {
    val beaconChain = createBeaconChainAtHeight(chainHeight)
    val blockChain = QbftBlockchainAdapter(beaconChain)
    return QbftMessageProcessor(
      blockChain = blockChain,
      validatorProvider = validatorProvider,
      localAddress = localAddress,
      bftEventQueue = bftEventQueue,
      messageDecoder = messageDecoder,
    )
  }

  private fun createBeaconChainAtHeight(targetHeight: ULong): InMemoryBeaconChain {
    val beaconChain = inMemoryBeaconChainFromGenesis()
    var currentState = beaconChain.getLatestBeaconState()

    // Advance chain to target height
    for (blockNumber in 1UL..targetHeight) {
      val beaconBlock = DataGenerators.randomBeaconBlock(
        blockNumber,
      )
      val sealedBlock = SealedBeaconBlock(beaconBlock, emptySet())
      currentState = currentState.copy(beaconBlockHeader = beaconBlock.beaconBlockHeader)

      beaconChain.newBeaconChainUpdater().run {
        putBeaconState(currentState)
        putSealedBeaconBlock(sealedBlock)
        commit()
      }
    }

    return beaconChain
  }

  private fun inMemoryBeaconChainFromGenesis(): InMemoryBeaconChain {
    val (beaconState, sealedBlock) = DataGenerators.genesisState()
    return InMemoryBeaconChain(beaconState, sealedBlock)
  }

  @Test
  fun `should ignore old messages without adding to queue`() {
    val messageProcessor = createMessageProcessor(chainHeight = 100UL)

    val qbftMessage = createQbftMessage(50L)
    val result = messageProcessor.handleQbftMessage(qbftMessage).get()

    assertThat(result.code).isEqualTo(ValidationResultCode.IGNORE)
    assertThat(result).isInstanceOf(Ignore::class.java)
    assertThat(bftEventQueue.isEmpty).isTrue()
  }

  @Test
  fun `should ignore future messages and add to queue`() {
    val messageProcessor = createMessageProcessor(chainHeight = 100UL)

    val qbftMessage = createQbftMessage(150L)
    val result = messageProcessor.handleQbftMessage(qbftMessage).get()

    assertThat(result.code).isEqualTo(ValidationResultCode.IGNORE)
    val bftEvent = bftEventQueue.poll(100, TimeUnit.MILLISECONDS)
    assertThat((bftEvent as QbftReceivedMessageEvent).message).isEqualTo(qbftMessage)
  }

  @Test
  fun `should accept current message from known validator when local is validator`() {
    val messageProcessor = createMessageProcessor(chainHeight = 100UL)
    whenever(validatorProvider.getValidatorsForBlock(any())).thenReturn(
      listOf(messageAuthor, localAddress),
    )

    val qbftMessage = createQbftMessage(100L)
    val result = messageProcessor.handleQbftMessage(qbftMessage).get()

    assertThat(result.code).isEqualTo(ValidationResultCode.ACCEPT)
    val bftEvent = bftEventQueue.poll(100, TimeUnit.MILLISECONDS)
    assertThat((bftEvent as QbftReceivedMessageEvent).message).isEqualTo(qbftMessage)
  }

  @Test
  fun `should reject current message from unknown validator`() {
    val knownValidator = Address.fromHexString("0xABCDEF1234567890123456789012345678901234")
    val messageProcessor = createMessageProcessor(chainHeight = 100UL)
    whenever(validatorProvider.getValidatorsForBlock(any())).thenReturn(
      listOf(knownValidator, localAddress), // messageAuthor is not in this list
    )

    val qbftMessage = createQbftMessage(100L)
    val result = messageProcessor.handleQbftMessage(qbftMessage).get()
    assertThat(result.code).isEqualTo(ValidationResultCode.REJECT)
    assertThat(bftEventQueue.isEmpty).isTrue()
  }

  @Test
  fun `should ignore current message when local is not a validator`() {
    val messageProcessor = createMessageProcessor(chainHeight = 100UL)
    whenever(validatorProvider.getValidatorsForBlock(any())).thenReturn(
      listOf(messageAuthor), // localAddress is not in the validator set
    )

    val qbftMessage = createQbftMessage(100L)
    val result = messageProcessor.handleQbftMessage(qbftMessage).get()

    assertThat(result.code).isEqualTo(ValidationResultCode.IGNORE)
    assertThat(bftEventQueue.isEmpty).isTrue()
  }

  @Test
  fun `should return invalid for malformed messages`() {
    val messageProcessor = createMessageProcessor(chainHeight = 100UL)
    val invalidMessageData = mock<MessageData>()
    val qbftMessage = mock<QbftMessage>()
    whenever(invalidMessageData.data).thenReturn(EMPTY)
    whenever(qbftMessage.data).thenReturn(invalidMessageData)

    val result = messageProcessor.handleQbftMessage(qbftMessage).get()

    assertThat(result.code).isEqualTo(ValidationResultCode.REJECT)
    assertThat(bftEventQueue.isEmpty).isTrue()
  }

  private fun createQbftMessage(sequenceNumber: Long): QbftMessage {
    val roundIdentifier = ConsensusRoundIdentifier(sequenceNumber, 1)
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
    return qbftMessage
  }
}
