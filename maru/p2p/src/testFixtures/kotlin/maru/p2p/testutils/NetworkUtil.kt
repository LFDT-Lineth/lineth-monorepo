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
  fun findFreePorts(count: Int): List<UInt> {
    // Keep all sockets open until the full set is collected so that concurrent callers
    // (other test forks or threads) see them as occupied and advance to different ports.
    val sockets = mutableListOf<ServerSocket>()
    try {
      while (sockets.size < count) {
        runCatching { ServerSocket(0) }
          .onSuccess { sockets.add(it) }
          .onFailure { throw RuntimeException("Could not find a free port", it) }
      }
      return sockets.map { it.localPort.toUInt() }
    } finally {
      sockets.forEach { runCatching { it.close() } }
    }
  }

  fun findFreePort(): UInt = findFreePorts(1).first()

  // Ports below 32768 are never auto-assigned by the kernel for port-0 binds (the OS ephemeral
  // range starts at 32768+ on Linux and 49152+ on macOS). Use this instead of findFreePort()
  // when the same port number must survive a service stop/restart within a single test — no
  // concurrent fork's port-0 bind can accidentally land on a port in this range.
  fun findStablePort(): UInt {
    for (port in 25000..31999) {
      runCatching { ServerSocket(port).also { it.close() } }.onSuccess { return port.toUInt() }
    }
    error("No free port found in range 25000..31999")
  }
}
