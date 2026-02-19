package net.consensys.zkevm.ethereum

import linea.contract.l1.LineaRollupContractVersion
import linea.testing.CommandResult
import linea.testing.Runner
import org.hyperledger.besu.datatypes.Address
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.regex.Matcher
import java.util.regex.Pattern

internal val lineaRollupAddressPattern = Pattern.compile(
  "^contract=LineaRollup(?:.*)? deployed: address=(0x[0-9a-fA-F]{40}) blockNumber=(\\d+)",
)
internal val l2MessageServiceAddressPattern = Pattern.compile(
  "^contract=L2MessageService(?:.*)? deployed: address=(0x[0-9a-fA-F]{40}) blockNumber=(\\d+)",
)

data class DeployedContract(
  val address: String,
  val blockNumber: Long,
)

fun getDeployedAddress(commandResult: CommandResult, addressPattern: Pattern): DeployedContract {
  val lines = commandResult.stdOutLines.toList().asReversed()
  return getDeployedAddress(lines, addressPattern)
}

fun getDeployedAddress(cmdStdoutLines: List<String>, addressPattern: Pattern): DeployedContract {
  val matcher: Matcher? = cmdStdoutLines
    .firstOrNull { line -> addressPattern.matcher(line).find() }
    ?.let { addressPattern.matcher(it).also { it.find() } }

  return matcher
    ?.let {
      val address = it.group(1)
      val deploymentBlockNumber = it.group(2).toLong()
      // validated address was correctly parsed
      Address.fromHexString(address)
      DeployedContract(address, deploymentBlockNumber)
    }
    ?: throw IllegalStateException("Couldn't extract contract address. Expecting pattern: $addressPattern")
}

private fun deployContract(
  command: String,
  env: Map<String, String> = emptyMap(),
  addressPattern: Pattern,
): SafeFuture<DeployedContract> {
  return Runner.executeCommand(
    command = command,
    envVars = env,
  )
    .thenApply { result ->
      if (result.exitCode != 0) {
        logCommand(result)
        throw IllegalStateException(
          "Command $command failed: " +
            "\nexitCode=${result.exitCode} " +
            "\nSTD_OUT: \n${result.stdOutStr}" +
            "\nSTD_ERROR: \n${result.stdErrStr}",
        )
      } else {
        runCatching { getDeployedAddress(result, addressPattern) }
          .onFailure { logCommand(result) }
          .getOrThrow()
      }
    }
}

fun makeDeployLineaRollup(
  deploymentPrivateKey: String? = null,
  operatorsAddresses: List<String>,
  contractVersion: LineaRollupContractVersion,
): SafeFuture<DeployedContract> {
  val env = mutableMapOf(
    "LINEA_ROLLUP_OPERATORS" to operatorsAddresses.joinToString(","),
    // "HARDHAT_DISABLE_CACHE" to "true"
  )
  deploymentPrivateKey?.let { env["DEPLOYMENT_PRIVATE_KEY"] = it }
  val command = when (contractVersion) {
    LineaRollupContractVersion.V6 -> "make deploy-linea-rollup-v6"
    LineaRollupContractVersion.V7 -> "make deploy-linea-rollup-v7"
    LineaRollupContractVersion.V8 -> "make deploy-linea-rollup-v8"
    // else -> throw IllegalArgumentException("Unsupported contract version: $contractVersion")
  }

  return deployContract(
    command = command,
    env = env,
    addressPattern = lineaRollupAddressPattern,
  )
}

fun makeDeployL2MessageService(
  deploymentPrivateKey: String? = null,
  anchorOperatorAddresses: String,
): SafeFuture<DeployedContract> {
  val env = mutableMapOf(
    "L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER" to anchorOperatorAddresses,
  )
  deploymentPrivateKey?.let { env["DEPLOYMENT_PRIVATE_KEY"] = it }

  return deployContract(
    command = "make deploy-l2messageservice",
    env = env,
    addressPattern = l2MessageServiceAddressPattern,
  )
}

fun logCommand(commandResult: CommandResult) {
  println("stdout:")
  println(commandResult.stdOutStr)
  println("stderr:")
  println(commandResult.stdErrStr)
  println("exit code: ${commandResult.exitCode}")
}

fun main() {
  SafeFuture.collectAll(
    makeDeployLineaRollup(
      L1AccountManager.generateAccount().privateKey,
      listOf("03dfa322A95039BB679771346Ee2dBfEa0e2B773"),
      LineaRollupContractVersion.V6,
    ),
    makeDeployL2MessageService(
      L2AccountManager.generateAccount().privateKey,
      "03dfa322A95039BB679771346Ee2dBfEa0e2B773",
    ),
  ).thenApply { addresses ->
    println("LineaRollup address: ${addresses[0]}")
    println("L2MessageService address: ${addresses[1]}")
  }.join()
}
