/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test

@Tag("AcceptanceTest")
class NamedSignerProviderServiceTest : LineaPluginTestBase() {
  override fun requestedPlugins(): List<String> =
    DEFAULT_REQUESTED_PLUGINS +
      "PackagedLineaSignerPlugin"

  override fun getTestCliOptions(): List<String> =
    TestCommandLineOptionsBuilder()
      .set("--plugin-linea-liveness-enabled=", "true")
      .set(
        "--plugin-linea-liveness-contract-address=",
        "0x0000000000000000000000000000000000000001",
      )
      .set("--plugin-linea-liveness-signer-type=", "CUSTOM")
      .set("--plugin-linea-liveness-signer-name=", "liveness-test")
      .set(
        "--plugin-linea-liveness-signer-address=",
        "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf",
      )
      .build()

  @Test
  fun `starts transaction selector with signer from separately packaged plugin`() {
    assertThat(minerNode).isNotNull()
  }
}
