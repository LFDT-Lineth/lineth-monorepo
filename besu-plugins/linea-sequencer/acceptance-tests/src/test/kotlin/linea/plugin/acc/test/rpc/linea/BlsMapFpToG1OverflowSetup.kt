/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import linea.plugin.acc.test.tests.web3j.generated.BLS12_MAP_FP_TO_G1
import org.apache.tuweni.bytes.Bytes

/**
 * Shared setup for tests that simulate PRECOMPILE_BLS_MAP_FP_TO_G1_EFFECTIVE_CALLS overflow.
 */
object BlsMapFpToG1OverflowSetup {

  /**
   * Test cases from
   * https://github.com/hyperledger/besu/blob/main/evm/src/test/resources/org/hyperledger/besu/evm/precompile/fp_to_g1.csv
   * One call to BLS12_MAP_FP_TO_G1 with any of these inputs triggers PRECOMPILE_BLS_MAP_FP_TO_G1_EFFECTIVE_CALLS=1.
   */
  val invalidInputs: Array<Bytes> =
    arrayOf(
      Bytes.fromHexString(
        "0000000000000000000000000000000014406e5bfb9209256a3820879a29ac2f" +
          "62d6aca82324bf3ae2aa7d3c54792043bd8c791fccdb080c1a52dc68b8b69350",
      ),
      Bytes.fromHexString(
        "000000000000000000000000000000000e885bb33996e12f07da69073e2c0cc8" +
          "80bc8eff26d2a724299eb12d54f4bcf26f4748bb020e80a7e3794a7b0e47a641",
      ),
      Bytes.fromHexString(
        "000000000000000000000000000000000ba1b6d79150bdc368a14157ebfe8b5f" +
          "691cf657a6bbe30e79b6654691136577d2ef1b36bfb232e3336e7e4c9352a8ed",
      ),
      Bytes.fromHexString(
        "000000000000000000000000000000000f12847f7787f439575031bcdb1f03cf" +
          "b79f942f3a9709306e4bd5afc73d3f78fd1c1fef913f503c8cbab58453fb7df2",
      ),
    )

  const val EXPECTED_OVERFLOW_MESSAGE: String =
    "Transaction line count for module PRECOMPILE_BLS_MAP_FP_TO_G1_EFFECTIVE_CALLS=1 is above the limit 0"

  fun encodeOverflowCall(contract: BLS12_MAP_FP_TO_G1, inputIndex: Int = 0): String {
    return contract
      .callBLS12_MAP_FP_TO_G1(invalidInputs[inputIndex].toArrayUnsafe())
      .encodeFunctionCall()
  }
}
