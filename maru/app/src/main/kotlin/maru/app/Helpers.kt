/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.ApiEndpointConfig
import maru.consensus.ElFork
import maru.consensus.ForksSchedule
import maru.consensus.NewBlockHandlerMultiplexer
import maru.consensus.blockimport.ElForkAwareBlockImporter
import maru.consensus.blockimport.FollowerBeaconBlockImporter
import maru.consensus.state.FinalizationProvider
import maru.executionlayer.ExecutionLayerFactory.buildExecutionLayerManager
import maru.web3j.TekuWeb3JClientFactory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient

object Helpers {
  fun createWeb3jClient(
    apiEndpointConfig: ApiEndpointConfig,
    log: Logger,
  ): Web3JClient =
    TekuWeb3JClientFactory
      .create(
        endpoint = apiEndpointConfig.endpoint,
        jwtPath = apiEndpointConfig.jwtSecretPath,
        timeout = apiEndpointConfig.timeout,
        log = log,
      )

  fun createBlockImportHandlers(
    elFork: ElFork,
    metricsFacade: MetricsFacade,
    finalizationStateProvider: FinalizationProvider,
    followerELNodeEngineApiWeb3JClients: Map<String, Web3JClient>,
  ): NewBlockHandlerMultiplexer {
    val elFollowersNewBlockHandlerMap =
      followerELNodeEngineApiWeb3JClients.mapValues { (followerName, web3JClient) ->
        val elFollowerExecutionLayerManager =
          buildExecutionLayerManager(
            web3JEngineApiClient = web3JClient,
            elFork = elFork,
            metricsFacade = metricsFacade,
          )
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
    metricsFacade: MetricsFacade,
    followerELNodeEngineApiWeb3JClients: Map<String, Web3JClient>,
    finalizationProvider: FinalizationProvider,
  ): NewBlockHandlerMultiplexer {
    val elFollowersNewBlockHandlerMap =
      followerELNodeEngineApiWeb3JClients.mapValues { (followerName, web3JClient) ->
        val elManagerMap =
          ElFork.entries.associateWith { elFork ->
            buildExecutionLayerManager(
              web3JEngineApiClient = web3JClient,
              elFork = elFork,
              metricsFacade = metricsFacade,
            )
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
