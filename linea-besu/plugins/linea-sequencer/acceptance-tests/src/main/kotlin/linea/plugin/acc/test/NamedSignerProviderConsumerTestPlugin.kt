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
import org.hyperledger.besu.plugin.BesuPlugin
import org.hyperledger.besu.plugin.ServiceManager

/** Acceptance-test consumer that resolves the signer factory after all plugins register. */
class NamedSignerProviderConsumerTestPlugin : BesuPlugin {
  private lateinit var serviceManager: ServiceManager

  override fun register(serviceManager: ServiceManager) {
    this.serviceManager = serviceManager
  }

  override fun start() {
    val provider = serviceManager.getService(NamedSignerProviderService::class.java).orElseThrow {
      IllegalStateException("NamedSignerProviderService not found in ServiceManager")
    }
    val signer = provider.create(SIGNER_NAME)
    providerClassLoader = provider.javaClass.classLoader
    resolvedPublicKey = signer.publicKey()
  }

  override fun stop() {}

  companion object {
    const val PLUGIN_NAME = "NamedSignerProviderConsumerTestPlugin"
    const val SIGNER_NAME = "liveness-test"

    @Volatile
    var providerClassLoader: ClassLoader? = null
      private set

    @Volatile
    var resolvedPublicKey: ByteArray? = null
      private set

    fun reset() {
      providerClassLoader = null
      resolvedPublicKey = null
    }
  }
}
