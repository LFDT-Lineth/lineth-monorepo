/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import linea.signing.NamedSignerProviderService
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test

@Tag("AcceptanceTest")
class NamedSignerProviderServiceTest : LineaPluginTestBase() {
  override fun requestedPlugins(): List<String> =
    DEFAULT_REQUESTED_PLUGINS +
      listOf("PackagedLineaSignerPlugin", NamedSignerProviderConsumerTestPlugin.PLUGIN_NAME)

  @AfterEach
  fun resetConsumer() {
    NamedSignerProviderConsumerTestPlugin.reset()
  }

  @Test
  fun `resolves signer factory registered by separately packaged plugin`() {
    assertThat(NamedSignerProviderConsumerTestPlugin.resolvedPublicKey).hasSize(64)
    assertThat(NamedSignerProviderConsumerTestPlugin.providerClassLoader)
      .isNotNull()
      .isNotSameAs(NamedSignerProviderService::class.java.classLoader)
  }
}
