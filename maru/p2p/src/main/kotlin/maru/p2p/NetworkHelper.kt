/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import java.net.Inet4Address
import java.net.InetAddress
import java.net.NetworkInterface

object NetworkHelper {
  val loopBackLastComparator =
    Comparator<InetAddress> { o1, o2 ->
      when {
        o1.isLoopbackAddress && !o2.isLoopbackAddress -> 1
        !o1.isLoopbackAddress && o2.isLoopbackAddress -> -1
        else -> 0
      }
    }

  fun listNetworkAddresses(excludeLoopback: Boolean = true): List<InetAddress> =
    NetworkInterface
      .getNetworkInterfaces()
      .toList()
      .flatMap { it.inetAddresses.toList() }
      .filter {
        if (excludeLoopback && it.isLoopbackAddress) false else true
      }.sortedWith(loopBackLastComparator)

  fun listIpsV4(excludeLoopback: Boolean = true): List<String> =
    listNetworkAddresses(excludeLoopback)
      .filter { it is Inet4Address }
      .map { it.hostAddress }

  fun selectIpV4ForP2P(
    targetIpV4: String,
    excludeLoopback: Boolean = true,
  ): String {
    val address =
      runCatching { Inet4Address.getByName(targetIpV4) }
        .getOrElse { throw IllegalArgumentException("Invalid targetIpV4=$targetIpV4", it) }

    if (address.isLoopbackAddress) {
      return targetIpV4
    }

    val ips = listIpsV4(excludeLoopback)
    check(ips.isNotEmpty()) { "No IPv4 addresses found on the local machine." }

    if (ips.contains(targetIpV4)) {
      return targetIpV4
    } else if (targetIpV4 == "0.0.0.0") {
      return ips.first()
    } else {
      val ipsString = ips.joinToString(separator = ",") { it }
      throw IllegalArgumentException("targetIpV4=$targetIpV4 not found in machine interfaces, available ips=$ipsString")
    }
  }
}
