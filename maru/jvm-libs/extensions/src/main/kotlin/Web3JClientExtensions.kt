/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.extensions

import org.web3j.protocol.http.HttpService
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient

fun Web3JClient.getEndpoint(): String =
  try {
    when (val service = this.web3jService) {
      is HttpService -> service.url
      else -> "unknown_endpoint"
    }
  } catch (e: Exception) {
    "unknown_endpoint"
  }
