/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import io.vertx.core.Vertx
import maru.config.ApiEndpointConfig
import maru.consensus.ElFork
import maru.consensus.ForksSchedule
import maru.consensus.NewBlockHandlerMultiplexer
import maru.consensus.blockimport.ElForkAwareBlockImporter
import maru.consensus.blockimport.FollowerBeaconBlockImporter
import maru.consensus.state.FinalizationProvider
import maru.executionlayer.ExecutionLayerFactory.buildExecutionLayerManager
import maru.executionlayer.client.EngineApiEndpoint
import maru.executionlayer.client.EngineApiTimeouts
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.Logger
import java.nio.file.Path

object Helpers {
  fun closeAll(vararg closeActions: () -> Unit) {
    var firstFailure: Throwable? = null
    closeActions.forEach { closeAction ->
      try {
        closeAction()
      } catch (error: Throwable) {
        val failure = firstFailure
        if (failure == null) {
          firstFailure = error
        } else if (failure !== error) {
          failure.addSuppressed(error)
        }
      }
    }
    firstFailure?.let { throw it }
  }

  fun createEngineApiEndpoint(
    vertx: Vertx,
    metricsFacade: MetricsFacade,
    apiEndpointConfig: ApiEndpointConfig,
    log: Logger,
  ): EngineApiEndpoint =
    EngineApiEndpoint.create(
      vertx = vertx,
      metricsFacade = metricsFacade,
      endpoint = apiEndpointConfig.endpoint,
      jwtSecretPath = apiEndpointConfig.jwtSecretPath?.let(Path::of),
      requestTimeouts = EngineApiTimeouts.provider(override = apiEndpointConfig.timeout),
      log = log,
    )

  fun createBlockImportHandlers(
    elFork: ElFork,
    finalizationStateProvider: FinalizationProvider,
    followerEngineApiEndpoints: Map<String, EngineApiEndpoint>,
  ): NewBlockHandlerMultiplexer {
    val elFollowersNewBlockHandlerMap =
      followerEngineApiEndpoints.mapValues { (followerName, engineApiEndpoint) ->
        val elFollowerExecutionLayerManager =
          buildExecutionLayerManager(engineApiEndpoint = engineApiEndpoint, elFork = elFork)
        FollowerBeaconBlockImporter.create(
          executionLayerManager = elFollowerExecutionLayerManager,
          finalizationStateProvider = finalizationStateProvider,
          importerName = followerName,
        )
      }
    return NewBlockHandlerMultiplexer(elFollowersNewBlockHandlerMap)
  }

  fun createForkAwareBlockImportHandlers(
    forksSchedule: ForksSchedule,
    followerEngineApiEndpoints: Map<String, EngineApiEndpoint>,
    finalizationProvider: FinalizationProvider,
  ): NewBlockHandlerMultiplexer {
    val elFollowersNewBlockHandlerMap =
      followerEngineApiEndpoints.mapValues { (followerName, engineApiEndpoint) ->
        val elManagerMap =
          ElFork.entries.associateWith { elFork ->
            buildExecutionLayerManager(engineApiEndpoint = engineApiEndpoint, elFork = elFork)
          }
        ElForkAwareBlockImporter(
          forksSchedule = forksSchedule,
          elManagerMap = elManagerMap,
          importerName = followerName,
          finalizationProvider = finalizationProvider,
        )
      }
    return NewBlockHandlerMultiplexer(elFollowersNewBlockHandlerMap)
  }
}
