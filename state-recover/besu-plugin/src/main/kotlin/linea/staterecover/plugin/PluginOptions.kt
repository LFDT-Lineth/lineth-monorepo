package linea.staterecover.plugin

import org.hyperledger.besu.datatypes.Address
import picocli.CommandLine
import java.net.URI
import java.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toKotlinDuration

data class PluginConfig(
  val lineaSequencerBeneficiaryAddress: Address,
  val l1SmartContractAddress: Address,
  val l1RpcEndpoint: URI,
  val blobscanEndpoint: URI,
  val shomeiEndpoint: URI,
  val l1PollingInterval: kotlin.time.Duration,
  val overridingRecoveryStartBlockNumber: ULong? = null
) {
  init {
    require(l1PollingInterval >= 1.seconds) { "Polling interval=$l1PollingInterval must be greater that 1s." }
  }
}

class PluginCliOptions {
  companion object {
    const val cliOptionsPrefix = "plugin-staterecovery"
  }

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-smart-contract-address"],
    description = ["L1 smart contract address"],
    required = true,
    converter = [AddressConverter::class],
    defaultValue = "\${env:L1_ROLLUP_CONTRACT_ADDRESS}"
  )
  lateinit var l1SmartContractAddress: Address

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-linea-sequencer-beneficiary-address"],
    description = ["Linea sequencer beneficiary address"],
    required = true,
    converter = [AddressConverter::class],
    defaultValue = "\${env:LINEA_SEQUENCER_BENEFICIARY_ADDRESS}"
  )
  lateinit var lineaSequencerBeneficiaryAddress: Address

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-rpc-endpoint"],
    description = ["L1 RPC endpoint"],
    required = true
  )
  lateinit var l1RpcEndpoint: URI

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-shomei-endpoint"],
    description = ["shomei (state manager) endpoint"],
    required = true
  )
  lateinit var shomeiEndpoint: URI

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-blobscan-endpoint"],
    description = ["blobscan api endpoint"],
    required = true
  )
  lateinit var blobscanEndpoint: URI

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-polling-interval"],
    description = ["L1 polling interval for new finalized blobs"],
    required = false
  )
  var l1PollingInterval: Duration = Duration.ofSeconds(12)

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-overriding-recovery-start-block-number"],
    description = [
      "Tries to force the recovery start block number to the given value. " +
        "This is mean for testing purposes, not production. Must be greater than or equal to 1."
    ],
    required = false
  )
  var overridingRecoveryStartBlockNumber: Long? = null

  fun getConfig(): PluginConfig {
    require(overridingRecoveryStartBlockNumber == null || overridingRecoveryStartBlockNumber!! >= 1) {
      "overridingRecoveryStartBlockNumber=$overridingRecoveryStartBlockNumber must be greater than or equal to 1"
    }
    return PluginConfig(
      lineaSequencerBeneficiaryAddress = lineaSequencerBeneficiaryAddress,
      l1SmartContractAddress = l1SmartContractAddress,
      l1RpcEndpoint = l1RpcEndpoint,
      blobscanEndpoint = blobscanEndpoint,
      shomeiEndpoint = shomeiEndpoint,
      l1PollingInterval = l1PollingInterval.toKotlinDuration(),
      overridingRecoveryStartBlockNumber = overridingRecoveryStartBlockNumber?.toULong()
    )
  }

  class AddressConverter : CommandLine.ITypeConverter<Address> {
    override fun convert(value: String): Address {
      return Address.fromHexString(value) ?: throw CommandLine.TypeConversionException(
        "Invalid address: $value"
      )
    }
  }
}
