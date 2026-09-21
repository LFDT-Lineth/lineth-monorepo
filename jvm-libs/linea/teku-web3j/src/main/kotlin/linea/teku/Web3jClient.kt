/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.teku

import okhttp3.OkHttpClient
import tech.pegasys.teku.ethereum.executionclient.ExecutionEngineClient

class Web3jClient internal constructor(
  val endpoint: String,
  private val httpClient: OkHttpClient,
  engineClient: ExecutionEngineClient,
) : ExecutionEngineClient by engineClient, AutoCloseable {
  override fun close() {
    httpClient.dispatcher.cancelAll()
    httpClient.dispatcher.executorService.shutdown()
    httpClient.connectionPool.evictAll()
  }
}
