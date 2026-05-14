/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import io.javalin.Javalin
import java.math.BigInteger
import java.util.concurrent.atomic.AtomicReference

/**
 * Minimal fake L1 JSON-RPC server for integration tests.
 *
 * Serves the real [linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly]
 *
 * Only [maru.finalization.LineaFinalizationProvider] contract call is supported: [finalizedL2BlockNumber] via
 * eth_call returning an ABI-encoded uint256. All other methods return safe no-op values.
 */
class FakeL1JsonRpcServer {
  companion object {
    private val METHOD_REGEX = Regex(""""method"\s*:\s*"([^"]+)"""")
    private val ID_REGEX = Regex(""""id"\s*:\s*(\w+)""")
  }

  private val finalizedBlockNumber = AtomicReference(0UL)

  fun setFinalizedL2BlockNumber(n: ULong) = finalizedBlockNumber.set(n)

  private val javalin =
    Javalin
      .create { config ->
        config.showJavalinBanner = false
      }.post("/*") { ctx ->
        val body = ctx.body()
        val method = METHOD_REGEX.find(body)?.groupValues?.get(1) ?: "unknown"
        val id = ID_REGEX.find(body)?.groupValues?.get(1) ?: "1"
        val result =
          when (method) {
            "eth_blockNumber" -> {
              """"0x1""""
            }

            "eth_call" -> {
              // ABI-encoded uint256: 32-byte big-endian.
              // Use java.lang.Long.toUnsignedString to avoid sign-extension when the
              // ULong value exceeds Long.MAX_VALUE.
              val hex =
                BigInteger(java.lang.Long.toUnsignedString(finalizedBlockNumber.get().toLong()))
                  .toString(16)
                  .padStart(64, '0')
              """"0x$hex""""
            }

            "eth_getLogs" -> {
              "[]"
            }

            "eth_chainId" -> {
              """"0x1""""
            }

            else -> {
              "null"
            }
          }
        ctx
          .contentType("application/json")
          .result("""{"jsonrpc":"2.0","id":$id,"result":$result}""")
      }

  fun start() = apply { javalin.start(0) }

  fun stop() {
    javalin.stop()
  }

  fun url(): String = "http://127.0.0.1:${javalin.port()}"
}
