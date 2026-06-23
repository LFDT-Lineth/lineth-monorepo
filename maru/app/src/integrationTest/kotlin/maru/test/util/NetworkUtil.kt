/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.util

import java.net.ServerSocket

object NetworkUtil {
  fun findFreePorts(count: Int): List<UInt> {
    val ports = mutableListOf<UInt>()
    val sockets = mutableListOf<ServerSocket>()
    var error: Throwable? = null
    while (ports.size < count && error == null) {
      runCatching {
        ServerSocket(0).also { socket ->
          sockets.add(socket)
          ports.add(socket.localPort.toUInt())
        }
      }.onFailure {
        error = it
      }
    }
    sockets.forEach { runCatching { it.close() } }
    if (error != null) {
      throw RuntimeException("Could not find a free port", error)
    }
    return ports
  }

  fun findFreePort(): UInt = findFreePorts(1).first()
}
