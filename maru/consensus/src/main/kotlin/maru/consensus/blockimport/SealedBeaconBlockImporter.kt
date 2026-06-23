/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.blockimport

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.flatMap
import com.github.michaelbull.result.mapError
import linea.kotlin.encodeHex
import maru.consensus.AsyncFunction
import maru.consensus.CallAndForgetFutureMultiplexer
import maru.consensus.state.StateTransition
import maru.consensus.validation.BeaconBlockValidatorFactory
import maru.consensus.validation.SealsVerifier
import maru.core.SealedBeaconBlock
import maru.database.BeaconChain
import maru.p2p.SealedBeaconBlockHandler
import maru.p2p.ValidationResult
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.util.log.LogUtil
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicBoolean

// This is basically Chain of Responsibility design pattern, except it doesn't allow multiple children
// Multiplexer class was created to address that
fun interface SealedBeaconBlockImporter<T> {
  fun importBlock(sealedBeaconBlock: SealedBeaconBlock): SafeFuture<T>
}

class NewSealedBeaconBlockHandlerMultiplexer<T>(
  handlersMap: Map<String, SealedBeaconBlockHandler<*>>,
  log: Logger = LogManager.getLogger(CallAndForgetFutureMultiplexer<*>::javaClass)!!,
) : CallAndForgetFutureMultiplexer<SealedBeaconBlock>(
  handlersMap = sealedBlockHandlersToGenericHandlers(handlersMap),
  log = log,
),
  SealedBeaconBlockHandler<Unit> {
  companion object {
    fun sealedBlockHandlersToGenericHandlers(
      handlersMap: Map<String, SealedBeaconBlockHandler<*>>,
    ): Map<String, AsyncFunction<SealedBeaconBlock, Unit>> =
      handlersMap.mapValues { newSealedBlockHandler ->
        {
          newSealedBlockHandler.value.handleSealedBlock(it).thenApply { }
        }
      }
  }

  override fun Logger.logError(
    handlerName: String,
    input: SealedBeaconBlock,
    ex: Exception,
  ) {
    this.error(
      "New sealed block handler $handlerName failed processing" +
        "blockHash=${input.beaconBlock.beaconBlockHeader.hash}, number=${input.beaconBlock.beaconBlockHeader.number} " +
        "executionPayloadBlockNumber=${input.beaconBlock.beaconBlockBody.executionPayload.blockNumber}!",
      ex,
    )
  }

  override fun handleSealedBlock(sealedBeaconBlock: SealedBeaconBlock): SafeFuture<Unit> = handle(sealedBeaconBlock)
}

/**
 * Responsible for the following steps
 * 1. state transition of node's BeaconChain
 * 2. new block import into an EL node
 * The import is not transactional, note all or nothing.
 * Steps 1 and 2 are performed sequentially. As long as Step 1 is successful, the block is considered imported.
 * Step 2 is fire and forget.
 */
class TransactionalSealedBeaconBlockImporter(
  private val beaconChain: BeaconChain,
  private val stateTransition: StateTransition,
  private val beaconBlockImporter: BeaconBlockImporter,
) : SealedBeaconBlockImporter<ValidationResult> {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  override fun importBlock(sealedBeaconBlock: SealedBeaconBlock): SafeFuture<ValidationResult> {
    val clBlockNumber = sealedBeaconBlock.beaconBlock.beaconBlockHeader.number
    val elBLockNumber = sealedBeaconBlock.beaconBlock.beaconBlockBody.executionPayload.blockNumber
    log.debug(
      "Importing clBlockNumber={} elBlockNumber={}",
      clBlockNumber,
      elBLockNumber,
    )
    val stateTransition =
      try {
        stateTransition.processBlock(sealedBeaconBlock.beaconBlock)
      } catch (ex: Throwable) {
        log.error(
          "State transition threw an exception clBlockNumber={} elBlockNumber={}",
          clBlockNumber,
          elBLockNumber,
          ex,
        )
        return SafeFuture.failedFuture(ex)
      }

    return stateTransition
      .thenApply { resultingState ->
        beaconChain.newBeaconChainUpdater().use { updater ->
          updater
            .putBeaconState(resultingState)
            .putSealedBeaconBlock(sealedBeaconBlock)
            .commit()
        }
        log.trace(
          "DB Import complete clBlockNumber={} elBlockNumber={}",
          clBlockNumber,
          elBLockNumber,
        )
        resultingState
      }.thenPeek { resultingState ->
        // Fire and forget
        beaconBlockImporter
          .importBlock(resultingState, sealedBeaconBlock.beaconBlock)
          .whenException { e ->
            // Block import doesn't participate in the validation, so we want it to complete, yet ignore its result
            log.warn(
              "Failure importing a valid CL block! clBlockNumber={}, elBlockNumber={}",
              clBlockNumber,
              elBLockNumber,
              e,
            )
          }
      }.thenApply {
        ValidationResult.Companion.Valid as ValidationResult
      }.exceptionally { ex ->
        log.error(
          "DB Import failed clBlockNumber={} elBlockNumber={}",
          clBlockNumber,
          elBLockNumber,
          ex,
        )
        ValidationResult.Companion.Invalid(ex.message!!, ex.cause)
      }
  }
}

/**
 * Verifies the seal and delegates to another beaconBlockImporter
 */
class ValidatingSealedBeaconBlockImporter(
  private val sealsVerifier: SealsVerifier,
  private val beaconBlockImporter: SealedBeaconBlockImporter<ValidationResult>,
  private val beaconBlockValidatorFactory: BeaconBlockValidatorFactory,
) : SealedBeaconBlockImporter<ValidationResult> {
  companion object {
    fun Result<Unit, String>.toDomain(): ValidationResult =
      when (this) {
        is Ok -> ValidationResult.Companion.Valid
        is Err -> ValidationResult.Companion.Invalid(this.error, null)
      }
  }

  private val log = LogManager.getLogger(this.javaClass)
  private val shouldLog = AtomicBoolean(true)

  override fun importBlock(sealedBeaconBlock: SealedBeaconBlock): SafeFuture<ValidationResult> {
    try {
      val beaconBlock = sealedBeaconBlock.beaconBlock
      val beaconBlockHeader = beaconBlock.beaconBlockHeader
      LogUtil.throttledLog(
        log::info,
        "block received: clBlockNumber=${beaconBlockHeader.number} " +
          "elBlockNumber=${beaconBlock.beaconBlockBody.executionPayload.blockNumber} " +
          "clBlockHash=${beaconBlockHeader.hash.encodeHex()}",
        shouldLog,
        30,
      )
      val blockValidators =
        beaconBlockValidatorFactory
          .createValidatorForBlock(beaconBlockHeader)
      return sealsVerifier
        .verifySeals(sealedBeaconBlock.commitSeals, beaconBlockHeader)
        .thenComposeCombined(
          blockValidators.validateBlock(beaconBlock),
        ) { sealsVerificationResult, blockValidationResult ->
          val combinedValidationResult =
            sealsVerificationResult.flatMap { blockValidationResult.mapError { it.message } }
          when (combinedValidationResult) {
            is Ok -> {
              log.debug(
                "block validated: clBlockNumber={} elBlockNumber={} clBlockHash={}",
                beaconBlockHeader.number,
                sealedBeaconBlock.beaconBlock.beaconBlockBody.executionPayload.blockNumber,
                beaconBlockHeader.hash.encodeHex(),
              )
              beaconBlockImporter.importBlock(sealedBeaconBlock).thenApply { it }
            }

            is Err -> {
              log.error(
                "validation failed: clBlockNumber={} elBlockNumber={} clBlockHash={} error={}",
                sealedBeaconBlock.beaconBlock.beaconBlockHeader.number,
                sealedBeaconBlock.beaconBlock.beaconBlockBody.executionPayload.blockNumber,
                sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash
                  .encodeHex(),
                combinedValidationResult.error,
              )
              SafeFuture.completedFuture(combinedValidationResult.toDomain())
            }
          }
        }.whenException {
          log.error(
            "exception during block import: clBlockNumber={} elBlockNumber={}  clBlockHash={} errorMessage={}",
            sealedBeaconBlock.beaconBlock.beaconBlockHeader.number,
            sealedBeaconBlock.beaconBlock.beaconBlockBody.executionPayload.blockNumber,
            sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash
              .encodeHex(),
            it.message,
            it,
          )
        }
    } catch (ex: Throwable) {
      log.error(
        "exception during block import: clBlockNumber={} elBlockNumber={} clBlockHash={} errorMessage={}",
        sealedBeaconBlock.beaconBlock.beaconBlockHeader.number,
        sealedBeaconBlock.beaconBlock.beaconBlockBody.executionPayload.blockNumber,
        sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash
          .encodeHex(),
        ex.message,
        ex,
      )
      throw ex
    }
  }
}
