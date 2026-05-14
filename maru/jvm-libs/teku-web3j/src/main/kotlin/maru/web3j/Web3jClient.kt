/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.web3j

import org.web3j.protocol.Web3jService
import tech.pegasys.teku.ethereum.events.ExecutionClientEventsChannel
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import tech.pegasys.teku.infrastructure.time.TimeProvider

internal class Web3jClient(
  eventLog: tech.pegasys.teku.infrastructure.logging.EventLogger,
  web3jService: Web3jService,
  timeProvider: TimeProvider,
  executionClientEventsPublisher: ExecutionClientEventsChannel,
  nonCriticalMethods: Set<String> = emptySet(),
) : Web3JClient(eventLog, timeProvider, executionClientEventsPublisher, nonCriticalMethods) {
  init {
    initWeb3jService(web3jService)
  }
}
