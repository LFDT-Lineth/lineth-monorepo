/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.teku

import org.web3j.protocol.Web3j
import org.web3j.protocol.Web3jService
import tech.pegasys.teku.ethereum.events.ExecutionClientEventsChannel
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import tech.pegasys.teku.infrastructure.time.TimeProvider

/**
 * [web3jService] (a [Jackson2HttpService]) is only correct for the engine-api schema types (see
 * that class's doc). Plain `eth_*` calls made through the base class's [getEth1Web3j] (e.g. Teku's
 * own [tech.pegasys.teku.ethereum.executionclient.web3j.Web3JExecutionEngineClient.getPowChainHead])
 * need web3j's own response types (e.g. [org.web3j.protocol.core.methods.response.EthBlock]),
 * which rely on Jackson-3-only annotations invisible to [Jackson2HttpService]'s classic Jackson 2
 * mapper. [eth1Web3j] is a separately-built, vanilla web3j client (same transport/auth, web3j's own
 * default Jackson 3 mapper) so those calls resolve correctly instead.
 */
internal class Web3jClient(
  eventLog: tech.pegasys.teku.infrastructure.logging.EventLogger,
  web3jService: Web3jService,
  timeProvider: TimeProvider,
  executionClientEventsPublisher: ExecutionClientEventsChannel,
  // TODO: Remove once Teku doesn't cause Web3j 5.x.x vs 6.x.x and Jackson 2 vs Jackson 3 API conflict
  private val eth1Web3j: Web3j,
  nonCriticalMethods: Set<String> = emptySet(),
) : Web3JClient(eventLog, timeProvider, executionClientEventsPublisher, nonCriticalMethods) {
  init {
    initWeb3jService(web3jService)
  }

  override fun getEth1Web3j(): Web3j = eth1Web3j
}
