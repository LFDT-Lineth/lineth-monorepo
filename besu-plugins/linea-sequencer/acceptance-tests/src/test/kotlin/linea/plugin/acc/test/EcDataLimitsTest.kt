/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import org.web3j.utils.Numeric
import java.math.BigInteger
import java.util.function.BiFunction
import java.util.stream.Stream

class EcDataLimitsTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      // disable line count validation to accept excluded precompile txs in the txpool
      .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "false")
      // set the module limits file
      .set("--plugin-linea-module-limit-file-path=", getResourcePath("/moduleLimits.toml"))
      .build()
  }

  override fun getCliqueOptions(): GenesisConfigurationFactory.CliqueOptions {
    return GenesisConfigurationFactory.CliqueOptions(
      BLOCK_PERIOD_SECONDS,
      GenesisConfigurationFactory.CliqueOptions.DEFAULT.epochLength(),
      false,
    )
  }

  /**
   * Tests the EcPairing limits, that are the number of times a certain circuit may be invoked in a
   * single block.
   *
   * @param nTransactions the number of transactions to try to include in the same block. The last
   *     one is not supposed to fit as it exceeds the limit, thus it is included in the next block
   * @param input a function that generates the input data for each transaction
   * @param target the expected string to be found in the blocks log
   * @throws Exception if an error occurs during the test
   */
  @ParameterizedTest
  @MethodSource("ecPairingLimitsTestSource")
  fun ecPairingLimitsTest(
    nTransactions: Int,
    input: BiFunction<Int, Int, String>,
    target: String,
  ) {
    // Deploy the EcPairing contract
    val ecPairing = deployEcPairing()

    // Create an account to send the transactions
    val ecPairingSender = accounts.createAccount("ecPairingSender")

    // Fund the account using secondary benefactor
    val fundTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, ecPairingSender, 1, BigInteger.ZERO)
      .execute(minerNode.nodeRequests())
      .toHexString()
    // Verify that the transaction for transferring funds was successful
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(fundTxHash))

    val txHashes = Array<String?>(nTransactions) { null }
    for (i in 0 until nTransactions) {
      // With decreasing nonce we force the transactions to be included in the same block
      // i     = 0                , 1                , ..., nTransactions - 1
      // nonce = nTransactions - 1, nTransactions - 2, ..., 0
      val nonce = nTransactions - 1 - i

      // Craft the transaction data
      val encodedCallEcPairing = encodedCallEcPairing(
        ecPairing,
        ecPairingSender,
        nonce,
        Bytes.fromHexString(input.apply(i, nTransactions)),
      )

      // Send the transaction
      val web3j = minerNode.nodeRequests().eth()
      val resp = web3j.ethSendRawTransaction(Numeric.toHexString(encodedCallEcPairing)).send()

      // Store the transaction hash
      txHashes[nonce] = resp.transactionHash
    }

    // Transfer used as sentry to ensure a new block is mined
    val transferTxHash = accountTransactions
      .createTransfer(
        accounts.primaryBenefactor,
        accounts.secondaryBenefactor,
        1,
        BigInteger.ONE, // nonce is 1 as primary benefactor also deploys the contract
      )
      .execute(minerNode.nodeRequests())
    // Wait for the sentry to be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()))

    // Assert that all the transactions involving the EcPairing precompile, but the last one, were
    // included in the same block
    assertTransactionsMinedInSameBlock(
      minerNode.nodeRequests().eth(),
      txHashes.toList().subList(0, nTransactions - 1).filterNotNull(),
    )

    // Assert that the last transaction was included in another block
    assertTransactionsMinedInSeparateBlocks(
      minerNode.nodeRequests().eth(),
      listOf(txHashes[0]!!, txHashes[nTransactions - 1]!!),
    )

    asserLogsContain(target)
  }

  /**
   * Tests the EcAdd PRECOMPILE_ECADD_EFFECTIVE_CALLS limit, that is the number of times the
   * corresponding circuit may be invoked in a single block.
   */
  @Test
  fun ecAddLimitTest() {
    val moduleLimits = ModuleLineCountValidator.createLimitModules(getResourcePath("/moduleLimits.toml"))
    val PRECOMPILE_ECADD_EFFECTIVE_CALLS = moduleLimits["PRECOMPILE_ECADD_EFFECTIVE_CALLS"]!!

    /*
     * nTransactions: the number of transactions to try to include in the same block.
     *     The last one is not supposed to fit as it exceeds the limit, thus it is
     *     included in the next block. Note that in this specific test more than one
     *     call to the ECADD precompile is executed within the same transaction to
     *     reach the limit with a smaller number of transaction
     *
     * input: input data for each transaction
     * target: the expected string to be found in the blocks log
     */
    val callsPerTransaction = 32
    val nTransactions = PRECOMPILE_ECADD_EFFECTIVE_CALLS / callsPerTransaction + 1
    val input =
      "070375d4eec4f22aa3ad39cb40ccd73d2dbab6de316e75f81dc2948a996795d5" +
        "041b98f07f44aa55ce8bd97e32cacf55f1e42229d540d5e7a767d1138a5da656" +
        "185f6f5cf93c8afa0461a948c2da7c403b6f8477c488155dfa8d2da1c62517b8" +
        "13d83d7a51eb18fdb51225873c87d44f883e770ce2ca56c305d02d6cb99ca5b8"
    val target =
      "Cumulated line count for module PRECOMPILE_ECADD_EFFECTIVE_CALLS=" +
        (PRECOMPILE_ECADD_EFFECTIVE_CALLS + callsPerTransaction) +
        " is above the limit " +
        PRECOMPILE_ECADD_EFFECTIVE_CALLS +
        ", stopping selection"

    // Deploy the EcAdd contract
    val ecAdd = deployEcAdd()

    // Create an account to send the transactions
    val ecAddSender = accounts.createAccount("ecAddSender")

    // Fund the account using secondary benefactor
    val fundTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, ecAddSender, 1, BigInteger.ZERO)
      .execute(minerNode.nodeRequests())
      .toHexString()
    // Verify that the transaction for transferring funds was successful
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(fundTxHash))

    val txHashes = Array<String?>(nTransactions) { null }
    for (i in 0 until nTransactions) {
      // With decreasing nonce we force the transactions to be included in the same block
      // i     = 0                , 1                , ..., nTransactions - 1
      // nonce = nTransactions - 1, nTransactions - 2, ..., 0
      val nonce = nTransactions - 1 - i

      // Craft the transaction data
      val encodedCallEcAdd = encodedCallEcAdd(ecAdd, ecAddSender, nonce, Bytes.fromHexString(input))

      // Send the transaction
      val web3j = minerNode.nodeRequests().eth()
      val resp = web3j.ethSendRawTransaction(Numeric.toHexString(encodedCallEcAdd)).send()

      // Store the transaction hash
      txHashes[nonce] = resp.transactionHash
    }

    // Transfer used as sentry to ensure a new block is mined
    val transferTxHash = accountTransactions
      .createTransfer(
        accounts.primaryBenefactor,
        accounts.secondaryBenefactor,
        1,
        BigInteger.ONE, // nonce is 1 as primary benefactor also deploys the contract
      )
      .execute(minerNode.nodeRequests())
    // Wait for the sentry to be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()))

    // Assert that all the transactions involving the EcPairing precompile, but the last one, were
    // included in the same block
    assertTransactionsMinedInSameBlock(
      minerNode.nodeRequests().eth(),
      txHashes.toList().subList(0, nTransactions - 1).filterNotNull(),
    )

    // Assert that the last transaction was included in another block
    assertTransactionsMinedInSeparateBlocks(
      minerNode.nodeRequests().eth(),
      listOf(txHashes[0]!!, txHashes[nTransactions - 1]!!),
    )

    asserLogsContain(target)
  }

  /**
   * Tests the EcMul PRECOMPILE_ECMUL_EFFECTIVE_CALLS limit, that is the number of times
   * the corresponding circuit may be invoked in a single block.
   */
  @Test
  fun ecMulLimitTest() {
    val moduleLimits = ModuleLineCountValidator.createLimitModules(getResourcePath("/moduleLimits.toml"))
    val PRECOMPILE_ECMUL_EFFECTIVE_CALLS = moduleLimits["PRECOMPILE_ECMUL_EFFECTIVE_CALLS"]!!

    /*
     * nTransactions: the number of transactions to try to include in the same block.
     *     The last one is not supposed to fit as it exceeds the limit, thus it is
     *     included in the next block
     * input: input data for each transaction
     * target: the expected string to be found in the blocks log
     */
    val nTransactions = PRECOMPILE_ECMUL_EFFECTIVE_CALLS + 1
    val input =
      "030644e72e131a029b85045b68181585d97816a916871ca8d3c208c16d87cfd3" +
        "15ed738c0e0a7c92e7845f96b2ae9c0a68a6a449e3538fc7ff3ebf7a5a18a2c4" +
        "0000000000000000000000000000000000000000000000000000000000000001"
    val target =
      "Cumulated line count for module PRECOMPILE_ECMUL_EFFECTIVE_CALLS=" +
        (PRECOMPILE_ECMUL_EFFECTIVE_CALLS + 1) +
        " is above the limit " +
        PRECOMPILE_ECMUL_EFFECTIVE_CALLS +
        ", stopping selection"

    // Deploy the EcMul contract
    val ecMul = deployEcMul()

    // Create an account to send the transactions
    val ecMulSender = accounts.createAccount("ecMulSender")

    // Fund the account using secondary benefactor
    val fundTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, ecMulSender, 1, BigInteger.ZERO)
      .execute(minerNode.nodeRequests())
      .toHexString()
    // Verify that the transaction for transferring funds was successful
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(fundTxHash))

    val txHashes = Array<String?>(nTransactions) { null }
    for (i in 0 until nTransactions) {
      // With decreasing nonce we force the transactions to be included in the same block
      // i     = 0                , 1                , ..., nTransactions - 1
      // nonce = nTransactions - 1, nTransactions - 2, ..., 0
      val nonce = nTransactions - 1 - i

      // Craft the transaction data
      val encodedCallEcMul = encodedCallEcMul(ecMul, ecMulSender, nonce, Bytes.fromHexString(input))

      // Send the transaction
      val web3j = minerNode.nodeRequests().eth()
      val resp = web3j.ethSendRawTransaction(Numeric.toHexString(encodedCallEcMul)).send()

      // Store the transaction hash
      txHashes[nonce] = resp.transactionHash
    }

    // Transfer used as sentry to ensure a new block is mined
    val transferTxHash = accountTransactions
      .createTransfer(
        accounts.primaryBenefactor,
        accounts.secondaryBenefactor,
        1,
        BigInteger.ONE, // nonce is 1 as primary benefactor also deploys the contract
      )
      .execute(minerNode.nodeRequests())
    // Wait for the sentry to be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()))

    // Assert that all the transactions involving the EcPairing precompile, but the last one, were
    // included in the same block
    assertTransactionsMinedInSameBlock(
      minerNode.nodeRequests().eth(),
      txHashes.toList().subList(0, nTransactions - 1).filterNotNull(),
    )

    // Assert that the last transaction was included in another block
    assertTransactionsMinedInSeparateBlocks(
      minerNode.nodeRequests().eth(),
      listOf(txHashes[0]!!, txHashes[nTransactions - 1]!!),
    )

    asserLogsContain(target)
  }

  /**
   * Tests the EcRecover PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS limit, that is the number of
   * times the corresponding circuit may be invoked in a single block.
   */
  @Test
  fun ecRecoverLimitTest() {
    val moduleLimits = ModuleLineCountValidator.createLimitModules(getResourcePath("/moduleLimits.toml"))
    val PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS = moduleLimits["PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS"]!!

    /*
     * nTransactions: the number of transactions to try to include in the same block.
     *     The last one is not supposed to fit as it exceeds the limit, thus it is
     *     included in the next block
     * input: input data for each transaction
     * target: the expected string to be found in the blocks log
     */
    val nTransactions = PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS + 1
    val input =
      Bytes.fromHexString(
        "279d94621558f755796898fc4bd36b6d407cae77537865afe523b79c74cc680b" +
          "000000000000000000000000000000000000000000000000000000000000001b" +
          "c2ff96feed8749a5ad1c0714f950b5ac939d8acedbedcbc2949614ab8af06312" +
          "1feecd50adc6273fdd5d11c6da18c8cfe14e2787f5a90af7c7c1328e7d0a2c42",
      )
    val target =
      "Cumulated line count for module PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS=" +
        (PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS + 1) +
        " is above the limit " +
        PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS +
        ", stopping selection"

    // Deploy the EcRecover contract
    val ecRecover = deployEcRecover()

    // Create an account to send the transactions
    val ecRecoverSender = accounts.createAccount("ecRecoverSender")
    // Fund the account using secondary benefactor
    val fundTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, ecRecoverSender, 1, BigInteger.ZERO)
      .execute(minerNode.nodeRequests())
      .toHexString()
    // Verify that the transaction for transferring funds was successful
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(fundTxHash))
    // send first tx (nonce=0) last one, so they will stay ready in the pool,
    // but will not be included in a block due to the nonce gap, doing this before
    // to avoid timing issues caused by slow tx sending calls that could cause flakiness
    val txHashes = (nTransactions downTo 0).map { nonce ->
      // Craft the transaction data
      val encodedCallEcRecover = encodedCallEcRecover(
        ecRecover,
        ecRecoverSender,
        nonce,
        input,
      )

      // Send the transaction
      val web3j = minerNode.nodeRequests().eth()
      val resp = web3j.ethSendRawTransaction(Numeric.toHexString(encodedCallEcRecover)).send()
      resp.transactionHash
    }
      .reversed()

    // Transfer used as sentry to ensure a new block is mined
    val transferTxHash = accountTransactions
      .createTransfer(
        accounts.primaryBenefactor,
        accounts.secondaryBenefactor,
        1,
        BigInteger.ONE, // nonce is 1 as primary benefactor also deploys the contract
      )
      .execute(minerNode.nodeRequests())
    // Wait for the sentry to be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()))

    // Assert that all the transactions involving the EcPairing precompile, but the last one, were
    // included in the same block
    assertTransactionsMinedInSameBlock(
      minerNode.nodeRequests().eth(),
      txHashes.toList().subList(0, nTransactions - 1).filterNotNull(),
    )

    // Assert that the last transaction was included in another block
    assertTransactionsMinedInSeparateBlocks(
      minerNode.nodeRequests().eth(),
      listOf(txHashes[0]!!, txHashes[nTransactions - 1]!!),
    )

    asserLogsContain(target)
  }

  companion object {
    @JvmStatic
    fun ecPairingLimitsTestSource(): Stream<Arguments> {
      val moduleLimits = ModuleLineCountValidator.createLimitModules(getResourcePath("/moduleLimits.toml"))
      val PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS = moduleLimits["PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS"]!!
      val PRECOMPILE_ECPAIRING_MILLER_LOOPS = moduleLimits["PRECOMPILE_ECPAIRING_MILLER_LOOPS"]!!
      val PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS = moduleLimits["PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS"]!!

      /*
      Structure of the input:
      Ax + Ay
      BxIm + BxRe
      ByIm + ByRe
       */

      // Valid pair requiring 1 Miller Loop and 1 final exponentiation
      val nonTrivial =
        "01395d002b3ca9180fb924650ef0656ead838fd027d487fed681de0d674c30da" +
          "097c3a9a072f9c85edf7a36812f8ee05e2cc73140749dcd7d29ceb34a8412188" +
          "2bd3295ff81c577fe772543783411c36f463676d9692ca4250588fbad0b44dc7" +
          "07d8d8329e62324af8091e3a4ffe5a57cb8664d1f5f6838c55261177118e9313" +
          "230f1851ba0d3d7d36c8603c7118c86bd2b6a7a1610c4af9e907cb702beff1d8" +
          "12843e703009c1c1a2f1088dcf4d91e9ed43189aa6327cae9a68be22a1aee5cb"

      // Valid pair requiring 1 G2 membership test
      val leftTrivialValid =
        "0000000000000000000000000000000000000000000000000000000000000000" +
          "0000000000000000000000000000000000000000000000000000000000000000" +
          "266152e278e5dab4e14f0d93a3e54550d08dc30ef4fe911257bd3e313864b859" +
          "22cebabf989f812c0a6e67362bcb83d55c6378a4f500ecc8a6a5518b3d1695e0" +
          "070a5a339edbbb67c35d0d44b3ffff6b5803b198af7645c892f6af2fa8abf6f2" +
          "117f82e731f61e688908fa2c831c6a1c7775e6f9cfd49e06d1d24d3d13e5936a"

      val arguments = mutableListOf<Arguments>()

      /*
      This test case produces PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS + 1 transactions performing one ECPAIRING.
      All ECPAIRING are well-formed and none of the points is ever the point at infinity,
      leading in total to:
      - 1 Miller loops
      - 0 G2 membership tests
      - 1 final exponentiations
      per pairing.

      In total:
      - PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS + 1 Miller loops (< PRECOMPILE_ECPAIRING_MILLER_LOOPS)
      - 0 G2 membership tests
      - PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS + 1 final exponentiations

      This final transaction exceeds the PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS and must be included in the next block.
       */
      arguments.add(
        Arguments.of(
          PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS + 1, // 1 final exponentiation per transaction
          BiFunction<Int, Int, String> { _, _ -> nonTrivial }, // 1 pair per transaction
          "Cumulated line count for module PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS=" +
            (PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS + 1) +
            " is above the limit " +
            PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS +
            ", stopping selection",
        ),
      )

      val nPairsPerTransaction = 8

      /*
      This test case produces PRECOMPILE_ECPAIRING_MILLER_LOOPS / nPairsPerTransaction + 1
      transactions each performing nPairsPerTransaction ECPAIRING's
      except the last transaction which requires only 1.
      All ECPAIRING are well-formed and none of the points is ever the point at infinity,
      leading in total to:
      - 1 Miller loops
      - 0 G2 membership tests
      - 1 final exponentiations
      per pairing.

      In total:
      - PRECOMPILE_ECPAIRING_MILLER_LOOPS + 1 Miller loops
      - 0 G2 membership tests
      - PRECOMPILE_ECPAIRING_MILLER_LOOPS / nPairsPerTransaction + 1 final exponentiations (< PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS)

      This final transaction exceeds the PRECOMPILE_ECPAIRING_MILLER_LOOPS and must be included in the next block.
       */
      arguments.add(
        Arguments.of(
          PRECOMPILE_ECPAIRING_MILLER_LOOPS / nPairsPerTransaction + 1,
          // nPairsPerTransaction Miller Loops per transaction except the last one which has 1
          BiFunction<Int, Int, String> { i, nTransactions ->
            nonTrivial.repeat(if (i < nTransactions - 1) nPairsPerTransaction else 1)
          },
          // nPairsPerTransaction pairs per transaction except the last one which has 1
          "Cumulated line count for module PRECOMPILE_ECPAIRING_MILLER_LOOPS=" +
            (PRECOMPILE_ECPAIRING_MILLER_LOOPS + 1) +
            " is above the limit " +
            PRECOMPILE_ECPAIRING_MILLER_LOOPS +
            ", stopping selection",
        ),
      )

      /*
      This test case produces PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS / nPairsPerTransaction + 1
      transactions each performing nPairsPerTransaction ECPAIRING's
      except the last transaction which requires only 1.
      All ECPAIRING are well-formed and the small point is always the point at infinity
      leading to:
      - 0 Miller loops
      - 1 G2 membership tests
      - 0 final exponentiations
      per pairing.

      In total:
      - 0 Miller loops
      - PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS + 1 G2 membership tests
      - 0 final exponentiations

      This final transaction exceeds the PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS and must be included in the next block.
       */
      arguments.add(
        Arguments.of(
          PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS / nPairsPerTransaction + 1,
          // nPairsPerTransaction g2 membership test per transaction except the last one which has
          // 1
          BiFunction<Int, Int, String> { i, nTransactions ->
            leftTrivialValid.repeat(if (i < nTransactions - 1) nPairsPerTransaction else 1)
          },
          // nPairsPerTransaction pairs per transaction except the last one which has 1
          "Cumulated line count for module PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS=" +
            (PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS + 1) +
            " is above the limit " +
            PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS +
            ", stopping selection",
        ),
      )

      /*
      Description of the test cases:

      - This method defines 3 test cases.
      - Each test case is defined by a tuple (nTransactions, input, target). See the test method signature for more details.
      - Each test case goal is crossing a limit independently. Specifically:
       * The first test case crosses the limit of final exponentiations.
       * The second test case crosses the limit of Miller loops.
       * The third test case crosses the limit of G2 membership tests.
        Note that while the first two test cases requires 2 circuits, the limits are crossed independently.
      - Each test cases generates at least two blocks, one for the transactions that fit in the limit and another for the
        transaction that exceeds the limit.
      - Due to how the corresponding test is structured, we observe exactly 4 blocks:
       * 1 including 1 transaction to deploy the EcPairing contract calling the precompile
       * 1 including 1 transaction to fund ecPairing sender
       * 1 including all the transactions that fit in the limit and the sentry transaction
       * 1 including the transaction that exceeds the limit
       */
      return arguments.stream()
    }
  }
}
