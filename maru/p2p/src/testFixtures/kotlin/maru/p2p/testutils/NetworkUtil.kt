/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.testutils

import java.net.ServerSocket

object NetworkUtil {
  // Ports below 32768 are never auto-assigned by the kernel for port-0 binds (the OS ephemeral
  // range starts at 32768+ on Linux and 49152+ on macOS). Staying in this range means that a
  // concurrent test fork using port-0 cannot accidentally steal a port we allocated here —
  // which matters when a port must survive a service stop/restart without a reservation socket.
  private val PORT_RANGE = 25000..31999

  fun findFreePorts(count: Int): List<UInt> {
    val found = mutableListOf<UInt>()
    for (port in PORT_RANGE) {
      if (found.size == count) break
      runCatching { ServerSocket(port).also { it.close() } }
        .onSuccess { found.add(port.toUInt()) }
    }
    if (found.size < count) {
      error("Could not find $count free ports in $PORT_RANGE (found ${found.size})")
    }
    return found
  }

  fun findFreePort(): UInt = findFreePorts(1).first()
}
