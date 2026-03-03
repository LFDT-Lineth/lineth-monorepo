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
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import java.io.IOException
import java.nio.file.Files
import java.nio.file.Path
import kotlin.io.path.exists
import kotlin.io.path.writeText
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction as DslTransaction

class EIP7702AuthListDenyReloadTest : LineaPluginPoSTestBase() {
  private lateinit var web3j: Web3j

  override fun getTestCliOptions(): List<String> {
    tempDenyList = tempDir.resolve("denyList.txt")
    if (!tempDenyList.exists()) {
      Files.createFile(tempDenyList)
    }
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-deny-list-path=", tempDenyList.toString())
      .set("--plugin-linea-delegate-code-tx-enabled=", "true")
      .build()
  }

  @BeforeEach
  override fun setup() {
    super.setup()
    web3j = minerNode.nodeRequests().eth()
  }

  @Test
  fun eip7702AuthListDenyCheckWorksAfterReload() {
    val sender = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val willBeDenied = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)

    val responseBeforeReload = sendEIP7702WithSeparateAuth(
      web3j = web3j,
      senderCredentials = sender,
      authSignerCredentials = willBeDenied,
      delegationAddress = Address.fromHexStringStrict(sender.address),
    )
    assertThat(responseBeforeReload.error).isNull()
    assertThat(responseBeforeReload.transactionHash).isNotNull()

    tempDenyList.writeText(willBeDenied.address)
    reloadPluginConfig()

    val responseAfterReload = sendEIP7702WithSeparateAuth(
      web3j = web3j,
      senderCredentials = sender,
      authSignerCredentials = willBeDenied,
      delegationAddress = Address.fromHexStringStrict(sender.address),
    )
    assertThat(responseAfterReload.transactionHash).isNull()
    assertThat(responseAfterReload.error.message).contains(
      "authorization authority ${willBeDenied.address} is blocked",
    )
  }

  @Test
  fun eip7702AuthListAllowCheckWorksAfterReload() {
    val sender = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val willBeAllowedAfterRestart = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)

    tempDenyList.writeText(willBeAllowedAfterRestart.address)
    reloadPluginConfig()

    val responseWhileDenied = sendEIP7702WithSeparateAuth(
      web3j = web3j,
      senderCredentials = sender,
      authSignerCredentials = willBeAllowedAfterRestart,
      delegationAddress = Address.fromHexStringStrict(sender.address),
    )
    assertThat(responseWhileDenied.transactionHash).isNull()
    assertThat(responseWhileDenied.error.message).contains(
      "authorization authority ${willBeAllowedAfterRestart.address} is blocked",
    )

    // writeText replaces the entire file, clearing the deny list
    tempDenyList.writeText("")
    reloadPluginConfig()

    val responseAfterRemoval = sendEIP7702WithSeparateAuth(
      web3j = web3j,
      senderCredentials = sender,
      authSignerCredentials = willBeAllowedAfterRestart,
      delegationAddress = Address.fromHexStringStrict(sender.address),
    )
    assertThat(responseAfterRemoval.error).isNull()
    assertThat(responseAfterRemoval.transactionHash).isNotNull()
  }

  private fun reloadPluginConfig() {
    val reqLinea = ReloadPluginConfigRequest()
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea).isEqualTo("Success")
  }

  class ReloadPluginConfigRequest : DslTransaction<String> {
    override fun execute(nodeRequests: NodeRequests): String {
      return try {
        Request<Any, ReloadPluginConfigResponse>(
          "plugins_reloadPluginConfig",
          listOf("net.consensys.linea.sequencer.txpoolvalidation.LineaTransactionPoolValidatorPlugin"),
          nodeRequests.web3jService,
          ReloadPluginConfigResponse::class.java,
        )
          .send()
          .result
      } catch (e: IOException) {
        throw RuntimeException(e)
      }
    }
  }

  class ReloadPluginConfigResponse : Response<String>()

  companion object {
    @JvmStatic
    @TempDir
    lateinit var tempDir: Path

    @JvmStatic
    lateinit var tempDenyList: Path
  }
}
