/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer

import maru.consensus.ElFork
import maru.executionlayer.client.CancunWeb3JJsonRpcExecutionLayerEngineApiClient
import maru.executionlayer.client.ExecutionLayerEngineApiClient
import maru.executionlayer.client.OsakaWeb3JJsonRpcExecutionLayerEngineApiClient
import maru.executionlayer.client.ParisWeb3JJsonRpcExecutionLayerEngineApiClient
import maru.executionlayer.client.PragueWeb3JJsonRpcExecutionLayerEngineApiClient
import maru.executionlayer.client.ShanghaiWeb3JJsonRpcExecutionLayerEngineApiClient
import maru.executionlayer.manager.ExecutionLayerManager
import maru.executionlayer.manager.JsonRpcExecutionLayerManager
import net.consensys.linea.metrics.MetricsFacade
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient

object ExecutionLayerFactory {
  fun buildExecutionLayerManager(
    web3JEngineApiClient: Web3JClient,
    elFork: ElFork,
    metricsFacade: MetricsFacade,
  ): ExecutionLayerManager =
    JsonRpcExecutionLayerManager(
      executionLayerEngineApiClient = buildExecutionEngineClient(
        web3JEngineApiClient = web3JEngineApiClient,
        elFork = elFork,
        metricsFacade = metricsFacade,
      ),
    )

  fun buildExecutionEngineClient(
    web3JEngineApiClient: Web3JClient,
    elFork: ElFork,
    metricsFacade: MetricsFacade,
  ): ExecutionLayerEngineApiClient =
    when (elFork) {
      ElFork.Paris ->
        ParisWeb3JJsonRpcExecutionLayerEngineApiClient(
          web3jClient = web3JEngineApiClient,
          metricsFacade = metricsFacade,
        )

      ElFork.Shanghai ->
        ShanghaiWeb3JJsonRpcExecutionLayerEngineApiClient(
          web3jClient = web3JEngineApiClient,
          metricsFacade = metricsFacade,
        )

      ElFork.Cancun ->
        CancunWeb3JJsonRpcExecutionLayerEngineApiClient(
          web3jClient = web3JEngineApiClient,
          metricsFacade = metricsFacade,
        )

      ElFork.Prague ->
        PragueWeb3JJsonRpcExecutionLayerEngineApiClient(
          web3jClient = web3JEngineApiClient,
          metricsFacade = metricsFacade,
        )
      ElFork.Osaka ->
        OsakaWeb3JJsonRpcExecutionLayerEngineApiClient(
          web3jClient = web3JEngineApiClient,
          metricsFacade = metricsFacade,
        )
    }
}
