/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import linea.plugin.acc.test.tests.web3j.generated.ExcludedPrecompiles
import org.apache.tuweni.bytes.Bytes32
import org.web3j.abi.datatypes.generated.Bytes8
import java.math.BigInteger
import java.nio.charset.StandardCharsets

/**
 * Utility object for encoding precompile contract calls.
 * Shared between ForcedTransactionTest and ExcludedPrecompilesLimitlessTest.
 */
object PrecompileCallEncoder {

  fun encodeRipemd160Call(contract: ExcludedPrecompiles, input: String = "I am not allowed here"): String {
    return contract
      .callRIPEMD160(input.toByteArray(StandardCharsets.UTF_8))
      .encodeFunctionCall()
  }

  fun encodeBlake2fCall(contract: ExcludedPrecompiles): String {
    return contract
      .callBlake2f(
        BigInteger.valueOf(12),
        listOf(
          Bytes32.fromHexString(
            "0x48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5",
          ).toArrayUnsafe(),
          Bytes32.fromHexString(
            "0xd182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b",
          ).toArrayUnsafe(),
        ),
        listOf(
          Bytes32.fromHexString(
            "0x6162630000000000000000000000000000000000000000000000000000000000",
          ).toArrayUnsafe(),
          Bytes32.ZERO.toArrayUnsafe(),
          Bytes32.ZERO.toArrayUnsafe(),
          Bytes32.ZERO.toArrayUnsafe(),
        ),
        listOf(Bytes8.DEFAULT.value, Bytes8.DEFAULT.value),
        true,
      )
      .encodeFunctionCall()
  }
}
