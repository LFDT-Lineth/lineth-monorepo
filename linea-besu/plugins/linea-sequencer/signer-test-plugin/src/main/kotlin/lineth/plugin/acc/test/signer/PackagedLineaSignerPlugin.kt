/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package lineth.plugin.acc.test.signer

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import lineth.signing.NamedSignerProviderService
import org.hyperledger.besu.plugin.BesuPlugin
import org.hyperledger.besu.plugin.ServiceManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.HexFormat

/** Acceptance-test provider loaded from a plugin JAR rather than the Besu application classpath. */
class PackagedLineaSignerPlugin : BesuPlugin {
  override fun register(serviceManager: ServiceManager) {
    serviceManager.addService(
      NamedSignerProviderService::class.java,
      TestNamedSignerProviderService(),
    )
  }

  override fun start() = Unit

  override fun stop() = Unit

  private class TestNamedSignerProviderService : NamedSignerProviderService {
    override fun get(name: String): Signer<Secp256k1Signature> {
      require(name == SIGNER_NAME) { "Unknown test signer: $name" }
      return TestSigner()
    }
  }

  private class TestSigner : Signer<Secp256k1Signature> {
    override fun publicKey(): ByteArray = PUBLIC_KEY.clone()

    override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> =
      SafeFuture.failedFuture(
        UnsupportedOperationException("The service-boundary test does not sign payloads"),
      )
  }

  companion object {
    const val PLUGIN_NAME = "PackagedLineaSignerPlugin"
    const val SIGNER_NAME = "liveness-test"
    val PUBLIC_KEY: ByteArray =
      HexFormat.of()
        .parseHex(
          "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798" +
            "483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8",
        )
  }
}
