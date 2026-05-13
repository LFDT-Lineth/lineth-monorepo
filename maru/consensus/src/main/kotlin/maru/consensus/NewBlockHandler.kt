/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import maru.core.BeaconBlock
import maru.core.SealedBeaconBlock
import maru.extensions.encodeHex
import maru.p2p.SealedBeaconBlockHandler
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentHashMap

class SealedBeaconBlockHandlerAdapter<T>(
  val adaptee: NewBlockHandler<T>,
) : SealedBeaconBlockHandler<T> {
  override fun handleSealedBlock(sealedBeaconBlock: SealedBeaconBlock): SafeFuture<T> =
    adaptee.handleNewBlock(sealedBeaconBlock.beaconBlock)
}

fun interface NewBlockHandler<T> {
  fun handleNewBlock(beaconBlock: BeaconBlock): SafeFuture<T>
}

typealias AsyncFunction<I, O> = (I) -> SafeFuture<O>

abstract class CallAndForgetFutureMultiplexer<I>(
  handlersMap: Map<String, AsyncFunction<I, *>>,
  protected val log: Logger = LogManager.getLogger(CallAndForgetFutureMultiplexer<*>::javaClass)!!,
) {
  private val handlersMap = ConcurrentHashMap(handlersMap)

  protected abstract fun Logger.logError(
    handlerName: String,
    input: I,
    ex: Exception,
  )

  fun addHandler(
    name: String,
    handler: AsyncFunction<I, Unit>,
  ) {
    handlersMap[name] = handler
  }

  fun handle(input: I): SafeFuture<Unit> {
    val handlerFutures: List<SafeFuture<Unit>> =
      handlersMap.map {
        val (handlerName, handler) = it
        SafeFuture
          .of {
            try {
              log.trace("calling handler='{}'", handlerName)
              handler(input).also {
                log.trace("handler='{}' completed successfully", handlerName)
              }
            } catch (ex: Exception) {
              log.logError(handlerName, input, ex)
              throw ex
            }
          }.thenApply { }
      }

    return SafeFuture
      .collectAll(handlerFutures.stream())
      .thenApply { }
  }
}

class NewBlockHandlerMultiplexer(
  handlersMap: Map<String, NewBlockHandler<*>>,
  log: Logger = LogManager.getLogger(NewBlockHandlerMultiplexer::class.java),
) : CallAndForgetFutureMultiplexer<BeaconBlock>(
  handlersMap = blockHandlersToGenericHandlers(handlersMap),
  log = log,
),
  NewBlockHandler<Unit> {
  companion object {
    fun blockHandlersToGenericHandlers(
      handlersMap: Map<String, NewBlockHandler<*>>,
    ): Map<String, AsyncFunction<BeaconBlock, Unit>> =
      handlersMap.mapValues { newSealedBlockHandler ->
        {
          newSealedBlockHandler.value.handleNewBlock(it).thenApply { }
        }
      }
  }

  override fun Logger.logError(
    handlerName: String,
    input: BeaconBlock,
    ex: Exception,
  ) {
    this.error(
      "new block handling failed: handler={} clBlockNumber={} elBlockNumber={} clBlockHash={} errorMessage={}",
      handlerName,
      input.beaconBlockHeader.number,
      input.beaconBlockBody.executionPayload.blockNumber,
      input.beaconBlockHeader.hash.encodeHex(),
      ex.message,
      ex,
    )
  }

  override fun handleNewBlock(beaconBlock: BeaconBlock): SafeFuture<Unit> = handle(beaconBlock)
}
