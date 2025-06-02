package net.consensys.linea

import org.hyperledger.besu.datatypes.Address
import picocli.CommandLine
import java.net.URI
import java.net.URL
import java.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toKotlinDuration

data class PluginConfig(
  val l1SmartContractAddress: Address,
  val l1RpcEndpoint: URL,
  val l1PollingInterval: kotlin.time.Duration,
) {
  init {
    require(l1PollingInterval >= 1.seconds) { "Polling interval=$l1PollingInterval must be greater that 1s." }
  }
}

class PluginCliOptions {
  @CommandLine.Option(
    names = ["--plugin-linea-l1-smart-contract-address"],
    description = ["L1 smart contract address"],
    required = true,
    converter = [AddressConverter::class],
  )
  lateinit var l1SmartContractAddress: Address

  @CommandLine.Option(
    names = ["--plugin-linea-l1-rpc-endpoint"],
    description = ["L1 RPC endpoint"],
    required = true,
  )
  lateinit var l1RpcEndpoint: String

  @CommandLine.Option(
    names = ["--plugin-linea-l1-polling-interval"],
    description = ["L1 polling interval"],
    required = false,
  )
  var l1PollingInterval: Duration = Duration.ofSeconds(12)

  fun getConfig(): PluginConfig {
    return PluginConfig(
      l1SmartContractAddress = l1SmartContractAddress,
      l1RpcEndpoint = URI(l1RpcEndpoint).toURL(),
      l1PollingInterval = l1PollingInterval.toKotlinDuration(),
    )
  }

  class AddressConverter : CommandLine.ITypeConverter<Address> {
    override fun convert(value: String): Address {
      return Address.fromHexString(value) ?: throw CommandLine.TypeConversionException(
        "Invalid address: $value",
      )
    }
  }
}
