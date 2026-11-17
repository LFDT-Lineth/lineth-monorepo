/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.testutils

import java.net.ServerSocket
import java.util.concurrent.ConcurrentHashMap
import kotlin.random.Random

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

  private const val STABLE_PORT_RANGE_START = 25000
  private const val STABLE_PORT_RANGE_END = 31999 // inclusive
  private val reservedStablePorts: MutableSet<Int> = ConcurrentHashMap.newKeySet()

  // Ports below 32768 are never auto-assigned by the kernel for port-0 binds nor used as source
  // ports of outgoing connections (the OS ephemeral range starts at 32768+ on Linux and 49152+ on
  // macOS). Use this instead of findFreePort() when the same port number must survive a service
  // stop/restart within a single test — no concurrent socket can accidentally land on a port in
  // this range while the service is down.
  // The scan starts at a random offset so that concurrent test forks don't race for the same ports,
  // and ports handed out are remembered so that concurrent tests in the same JVM never share one.
  fun findStablePorts(count: Int): List<UInt> {
    val rangeSize = STABLE_PORT_RANGE_END - STABLE_PORT_RANGE_START + 1
    val offset = Random.nextInt(rangeSize)
    val ports = mutableListOf<UInt>()
    for (i in 0 until rangeSize) {
      if (ports.size == count) break
      val port = STABLE_PORT_RANGE_START + (offset + i) % rangeSize
      if (!reservedStablePorts.add(port)) continue
      runCatching { ServerSocket(port).close() }
        .onSuccess { ports.add(port.toUInt()) }
        .onFailure { reservedStablePorts.remove(port) }
    }
    check(ports.size == count) {
      "Could only find ${ports.size}/$count free ports in range $STABLE_PORT_RANGE_START..$STABLE_PORT_RANGE_END"
    }
    return ports
  }

  fun findStablePort(): UInt = findStablePorts(1).first()
}
