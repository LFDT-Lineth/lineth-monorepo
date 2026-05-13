/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.net.Inet4Address
import java.net.InetAddress

class NetworkHelperTest {
  @Test
  fun `loopBackLastComparator favours not loopback`() {
    listOf<InetAddress>(
      Inet4Address.getByName("127.0.0.1"),
      Inet4Address.getByName("100.100.0.1"),
    ).sortedWith(NetworkHelper.loopBackLastComparator)
      .also { sorted ->
        assertThat(sorted).isEqualTo(
          listOf<InetAddress>(
            Inet4Address.getByName("100.100.0.1"),
            Inet4Address.getByName("127.0.0.1"),
          ),
        )
      }
  }

  @Test
  fun `listNetworkAddresses returns network addresses`() {
    NetworkHelper.listNetworkAddresses(excludeLoopback = true).also { addresses ->
      assertThat(addresses).allMatch { !it.isLoopbackAddress }
    }

    NetworkHelper.listNetworkAddresses(excludeLoopback = false).also { addresses ->
      assertThat(addresses).anyMatch { it.isLoopbackAddress }
    }
  }

  @Test
  fun `listIpV4 returns network addresses`() {
    NetworkHelper.listIpsV4(excludeLoopback = true).also { addresses ->
      assertThat(addresses).doesNotContain("127.0.0.1")
    }

    NetworkHelper.listIpsV4(excludeLoopback = false).also { addresses ->
      assertThat(addresses).contains("127.0.0.1")
    }
  }

  @Test
  fun `selectIpV4ForP2P returns targetIpV4 if present in local interfaces`() {
    val localIp = NetworkHelper.listIpsV4().first()
    val result = NetworkHelper.selectIpV4ForP2P(localIp)
    assertThat(result).isEqualTo(localIp)
  }

  @Test
  fun `selectIpV4ForP2P returns first IP if targetIpV4 is 0_0_0_0`() {
    val ips = NetworkHelper.listIpsV4()
    val result = NetworkHelper.selectIpV4ForP2P("0.0.0.0")
    assertThat(result).isEqualTo(ips.first())
  }

  @Test
  fun `selectIpV4ForP2P allows loopback when targetIpV4 is loopback`() {
    assertThat(NetworkHelper.selectIpV4ForP2P("127.0.0.1", excludeLoopback = true))
      .isEqualTo("127.0.0.1")

    assertThat(NetworkHelper.selectIpV4ForP2P("127.1", excludeLoopback = true))
      .isEqualTo("127.1")

    // decimal representation of loopback address, allowed by the spec
    assertThat(NetworkHelper.selectIpV4ForP2P("2130706433", excludeLoopback = true))
      .isEqualTo("2130706433")
  }

  @Test
  fun `selectIpV4ForP2P throws if targetIpV4 is not valid ip`() {
    val fakeIp = "192.0.A.A"
    assertThatThrownBy { NetworkHelper.selectIpV4ForP2P(fakeIp) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("Invalid targetIpV4=192.0.A.A")
  }

  @Test
  fun `selectIpV4ForP2P throws if targetIpV4 is not present`() {
    val fakeIp = "192.0.2.123" // TEST-NET-1, unlikely to exist
    assertThatThrownBy { NetworkHelper.selectIpV4ForP2P(fakeIp) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("targetIpV4=$fakeIp not found in machine interfaces")
  }
}
