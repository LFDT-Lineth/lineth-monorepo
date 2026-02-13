/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import linea.plugin.acc.test.rpc.ForcedTransactionParam
import linea.plugin.acc.test.rpc.GetForcedTransactionInclusionStatusRequest
import linea.plugin.acc.test.rpc.SendForcedRawTransactionRequest
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory
import org.junit.jupiter.api.Test
import java.math.BigInteger
import java.util.concurrent.TimeUnit

/**
 * Tests that a forced transaction exceeding the BLOCK_L2_L1_LOGS limit (16)
 * is correctly rejected with "TooManyLogs" status.
 *
 * For a log to count as L2→L1, it must:
 * 1. Be emitted by the configured bridge contract address
 * 2. Have a first topic matching the configured bridge topic
 */
class ForcedTransactionTooManyLogsTest : AbstractForcedTransactionTest() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/moduleLimitsLimitless.toml"),
      )
      .set("--plugin-linea-limitless-enabled=", "true")
      .set("--plugin-linea-deny-list-path=", getResourcePath("/defaultDenyList.txt"))
      .set("--plugin-linea-l1l2-bridge-contract=", LOG_EMITTER_ADDRESS.toHexString())
      .set("--plugin-linea-l1l2-bridge-topic=", BRIDGE_TOPIC)
      .build()
  }

  override fun getCliqueOptions(): GenesisConfigurationFactory.CliqueOptions {
    return GenesisConfigurationFactory.CliqueOptions(
      BLOCK_PERIOD_SECONDS,
      GenesisConfigurationFactory.CliqueOptions.DEFAULT.epochLength(),
      false,
    )
  }

  @Test
  fun forcedTransactionExceedingL2L1LogLimitIsRejectedWithTooManyLogs() {
    // Deploy the LogEmitter contract - should land at pre-computed LOG_EMITTER_ADDRESS
    val logEmitter = deployLogEmitter()

    // Verify the contract deployed at the expected address
    assertThat(logEmitter.contractAddress.lowercase())
      .withFailMessage(
        "LogEmitter deployed at %s but expected %s",
        logEmitter.contractAddress,
        LOG_EMITTER_ADDRESS.toHexString(),
      )
      .isEqualTo(LOG_EMITTER_ADDRESS.toHexString().lowercase())

    val sender = accounts.secondaryBenefactor

    val count = BigInteger.valueOf(BLOCK_L2_L1_LOGS_LIMIT + 1L)
    val topicBytes = Bytes32.fromHexString(BRIDGE_TOPIC).toArray()
    val callData = logEmitter.emitMultipleLogs(count, topicBytes).encodeFunctionCall()

    val rawTx = createSignedContractCall(sender, logEmitter.contractAddress, callData, 0)
    val forcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, rawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(1)

    // Wait for the forced transaction to be processed and rejected with TooManyLogs
    await()
      .atMost(60, TimeUnit.SECONDS)
      .pollInterval(1, TimeUnit.SECONDS)
      .untilAsserted {
        val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
          .execute(minerNode.nodeRequests())

        assertThat(statusResponse.result).isNotNull
        assertThat(statusResponse.result!!.inclusionResult).isEqualTo("TooManyLogs")
      }
  }

  companion object {
    private val DEPLOYER_ADDRESS = Address.fromHexString("0xfe3b557e8fb62b89f4916b721be55ceb828dbd73")

    private val LOG_EMITTER_ADDRESS: Address = Address.contractAddress(DEPLOYER_ADDRESS, 0L)

    private const val BRIDGE_TOPIC = "0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c"

    private const val BLOCK_L2_L1_LOGS_LIMIT = 16L
  }
}
