/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.consensus.qbft.MinimalQbftMessageDecoder.QbftMessageMetadata
import maru.consensus.qbft.adapters.QbftBlockchainAdapter
import maru.consensus.qbft.adapters.QbftValidatorProviderAdapter
import maru.consensus.qbft.adapters.toQbftReceivedMessageEvent
import maru.p2p.QbftMessageHandler
import maru.p2p.ValidationResult
import maru.p2p.ValidationResult.Companion.Ignore
import maru.p2p.ValidationResult.Companion.Invalid
import maru.p2p.ValidationResult.Companion.Valid
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.consensus.common.bft.BftEventQueue
import org.hyperledger.besu.consensus.qbft.core.messagedata.QbftV1
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import org.hyperledger.besu.datatypes.Address
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Processes QBFT messages received from the P2P network by applying a light validation, adding them to the
 * event queue if they are valid and returning the validation result to gossip if they are a current message.
 *
 * This mirrors the logic in Besu QbftController.processMessage but adapted for LibP2P message handling.
 *
 * Validation rules:
 * - Old messages (sequence < chainHeight): Ignored and not added to the event queue
 * - Future messages (sequence > chainHeight): Added to the event queue but not gossiped
 * - Current messages (sequence == chainHeight): Validated for author and local validator status and gossiped if valid
 */
class QbftMessageProcessor(
  private val blockChain: QbftBlockchainAdapter,
  private val validatorProvider: QbftValidatorProviderAdapter,
  private val localAddress: Address,
  private val bftEventQueue: BftEventQueue,
  private val messageDecoder: MinimalQbftMessageDecoder,
) : QbftMessageHandler<ValidationResult> {
  private val log = LogManager.getLogger(this.javaClass)

  /**
   * Called when a QBFT message arrives from the P2P network, **before** validation and queue insertion.
   * Parameters: (msgCode, sequenceNumber).
   *
   * This fires at the earliest possible point after P2P delivery — before the message enters
   * [BftEventQueue] — so the callback implementer can capture timestamps for P2P transit time.
   */
  var onMessageReceived: ((msgCode: Int, sequenceNumber: Long) -> Unit)? = null

  /**
   * Validates a QBFT message and determines whether it should be gossiped.
   *
   * @param qbftMessage The QBFT message to validate
   * @return A future containing the validation result
   */
  override fun handleQbftMessage(qbftMessage: QbftMessage): SafeFuture<ValidationResult> =
    try {
      val metadata = messageDecoder.deserialize(qbftMessage)
      log.debug(
        "P2P message received: type={} sequence={} round={} from={}",
        messageTypeName(metadata.messageCode),
        metadata.sequenceNumber,
        metadata.roundNumber,
        metadata.author,
      )
      // Fire as early as possible — before validation — to capture the true P2P arrival
      // timestamp. Validation overhead would skew phase-latency measurements.
      // Trade-off: rejected messages (unknown validators, old rounds) may record stale
      // timestamps, but this is rare and preferable to systematically late measurements.
      onMessageReceived?.invoke(metadata.messageCode, metadata.sequenceNumber)
      SafeFuture.completedFuture(processMessage(qbftMessage, metadata))
    } catch (e: Exception) {
      SafeFuture.completedFuture(Invalid("Failed to decode or validate message: ${e.message}"))
    }

  private fun processMessage(
    qbftMessage: QbftMessage,
    metadata: QbftMessageMetadata,
  ): ValidationResult {
    if (isMsgForCurrentHeight(metadata.sequenceNumber)) {
      val validators = validatorProvider.getValidatorsForBlock(blockChain.chainHeadHeader)
      return if (!isMsgFromKnownValidator(metadata.author, validators)) {
        Invalid("Message from unknown validator: ${metadata.author}")
      } else if (!isLocalNodeValidator(validators)) {
        Ignore("Local node is not a validator")
      } else {
        bftEventQueue.add(qbftMessage.toQbftReceivedMessageEvent())
        log.debug(
          "P2P message queued to BftEventQueue: type={} sequence={} round={}",
          messageTypeName(metadata.messageCode),
          metadata.sequenceNumber,
          metadata.roundNumber,
        )
        Valid
      }
    } else if (isMsgForFutureChainHeight(metadata.sequenceNumber)) {
      bftEventQueue.add(qbftMessage.toQbftReceivedMessageEvent())
      return Ignore("Future message, will be processed when chain reaches height ${metadata.sequenceNumber}")
    } else {
      return Ignore("Old message: sequence ${metadata.sequenceNumber} < height ${blockChain.chainHeadBlockNumber}")
    }
  }

  private fun isMsgFromKnownValidator(
    messageAuthor: Address,
    validators: Collection<Address>,
  ): Boolean = validators.contains(messageAuthor)

  private fun isMsgForCurrentHeight(sequenceNumber: Long): Boolean = sequenceNumber == blockChain.chainHeadBlockNumber

  private fun isMsgForFutureChainHeight(sequenceNumber: Long): Boolean =
    sequenceNumber > blockChain.chainHeadBlockNumber

  private fun isLocalNodeValidator(validators: Collection<Address>): Boolean = validators.contains(localAddress)

  private fun messageTypeName(code: Int): String =
    when (code) {
      QbftV1.PROPOSAL -> "PROPOSAL"
      QbftV1.PREPARE -> "PREPARE"
      QbftV1.COMMIT -> "COMMIT"
      QbftV1.ROUND_CHANGE -> "ROUND_CHANGE"
      else -> "UNKNOWN($code)"
    }
}
