package linea.staterecovery.plugin

import linea.domain.BlockParameter
import linea.domain.RetryConfig
import org.hyperledger.besu.datatypes.Address
import picocli.CommandLine
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.toKotlinDuration

data class PluginConfig(
  val lineaSequencerBeneficiaryAddress: Address,
  val lineaBlockGasLimit: ULong,
  val lineaBlockDifficulty: ULong,
  val l1SmartContractAddress: Address,
  val l1Endpoint: URI,
  val l1PollingInterval: kotlin.time.Duration,
  val l1GetLogsChunkSize: UInt,
  val l1EarliestSearchBlock: BlockParameter,
  val l1HighestSearchBlock: BlockParameter,
  val l1RequestSuccessBackoffDelay: kotlin.time.Duration,
  val l1RequestRetryConfig: RetryConfig,
  val blobscanEndpoint: URI,
  val blobScanRequestRetryConfig: RetryConfig,
  val shomeiEndpoint: URI,
  val overridingRecoveryStartBlockNumber: ULong? = null,
  val debugForceSyncStopBlockNumber: ULong? = null
) {
  init {
    require(l1PollingInterval >= 1.milliseconds) { "Polling interval=$l1PollingInterval must be greater than 1ms." }
  }
}

class PluginCliOptions {
  companion object {
    const val cliPluginPrefixName = "staterecovery"
    private const val cliOptionsPrefix = "plugin-$cliPluginPrefixName"
  }

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-linea-sequencer-beneficiary-address"],
    description = ["Linea sequencer beneficiary address"],
    required = true,
    converter = [AddressConverter::class],
    defaultValue = "\${env:LINEA_SEQUENCER_BENEFICIARY_ADDRESS}"
  )
  lateinit var lineaSequencerBeneficiaryAddress: Address

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-linea-block-gas-limit"],
    description = ["Linea Block gas limit. Default 2B (2_000_000_000)"],
    required = false,
    defaultValue = "\${env:LINEA_BLOCK_GAS_LIMIT}"
  )
  var lineaBlockGasLimit: Long = 2_000_000_000L

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-linea-block-difficulty"],
    description = ["Linea Block difficulty. Default 2"],
    required = false,
    defaultValue = "\${env:LINEA_BLOCK_DIFFICULTY}"
  )
  var lineaBlockDifficulty: Long = 2

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-smart-contract-address"],
    description = ["L1 smart contract address"],
    required = true,
    converter = [AddressConverter::class],
    defaultValue = "\${env:L1_ROLLUP_CONTRACT_ADDRESS}"
  )
  lateinit var l1SmartContractAddress: Address

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-endpoint"],
    description = ["L1 RPC endpoint"],
    required = true
  )
  lateinit var l1RpcEndpoint: URI

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-polling-interval"],
    defaultValue = "PT12S",
    description = ["L1 polling interval for new finalized blobs"],
    required = false
  )
  var l1PollingInterval: java.time.Duration = java.time.Duration.ofSeconds(12)

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-get-logs-chunk-size"],
    defaultValue = "10000",
    description = ["Chuck size (fromBlock..toBlock) for eth_getLogs initial search loop"],
    required = false
  )
  var l1GetLogsChunkSize: Int = 10_000

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-earliest-search-block"],
    defaultValue = "EARLIEST",
    description = [
      "Earliest L1 Block to search for new finalizations on startup.",
      "Optional, if defined it shall match L1 block with 1st finalization that supports recovery."
    ],
    converter = [BlockParameterConverter::class],
    required = false
  )
  var l1EarliestSearchBlock: BlockParameter = BlockParameter.Tag.EARLIEST

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-highest-search-block"],
    defaultValue = "FINALIZED",
    description = [
      "Highest L1 Block to search for new finalizations.",
      "Finalized is highly recommended, otherwise if state is reverted it may require a full resync. "
    ],
    converter = [BlockParameterConverter::class],
    required = false
  )
  var l1HighestSearchBlock: BlockParameter = BlockParameter.Tag.FINALIZED

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-success-backoff-delay"],
    description = [
      "L1 RPC api retry backoff delay, default none. ",
      "Request will fire as soon as previous response is received"
    ],
    required = false
  )
  var l1RequestSuccessBackoffDelay: java.time.Duration? = null

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-retry-backoff-delay"],
    defaultValue = "PT1S",
    description = ["L1 RPC api retry backoff delay, default 1s"],
    required = false
  )
  var l1RequestRetryBackoffDelay: java.time.Duration = java.time.Duration.ofSeconds(1)

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-retry-timeout"],
    description = [
      "L1 RPC api stop retrying as soon as timeout has elapsed or limit is reached",
      "default will retry indefinitely"
    ],
    required = false
  )
  var l1RequestRetryTimeout: java.time.Duration? = null

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-l1-retry-limit"],
    description = [
      "L1 RPC api stop retrying when limit is reached or timeout has elapsed",
      "default will retry indefinitely"
    ],
    required = false
  )
  var l1RequestRetryLimit: Int? = null

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
    names = ["--$cliOptionsPrefix-blobscan-retry-backoff-delay"],
    description = ["blobscan api retry backoff delay, default 1s"],
    required = false
  )
  var blobscanRequestRetryBackoffDelay: java.time.Duration = java.time.Duration.ofSeconds(1)

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-blobscan-retry-timeout"],
    description = [
      "Blobscan api stop retrying as soon as timeout has elapsed or limit is reached.",
      "default will retry indefinitely"
    ],
    required = false
  )
  var blobscanRequestRetryTimeout: java.time.Duration? = null

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-blobscan-retry-limit"],
    description = [
      "Blobscan api stop retrying when limit is reached or timeout has elapsed",
      "default will retry indefinitely"
    ],
    required = false
  )
  var blobscanRequestRetryLimit: Int? = null

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-overriding-recovery-start-block-number"],
    description = [
      "Tries to force the recovery start block number to the given value. " +
        "This is mean for testing purposes, not production. Must be greater than or equal to 1."
    ],
    defaultValue = "\${env:STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER}",
    required = false
  )
  var overridingRecoveryStartBlockNumber: Long? = null

  @CommandLine.Option(
    names = ["--$cliOptionsPrefix-debug-force-sync-stop-block-number"],
    description = [
      "Forces Besu to stop syncing at the given block number. " +
        "This is mean for testing purposes, not production. Must be greater than or equal to 1."
    ],
    defaultValue = "\${env:STATERECOVERY_DEBUG_FORCE_STOP_SYNC_BLOCK_NUMBER}",
    required = false
  )
  var debugForceSyncStopBlockNumber: Long? = null

  fun getConfig(): PluginConfig {
    require(overridingRecoveryStartBlockNumber == null || overridingRecoveryStartBlockNumber!! >= 1) {
      "overridingRecoveryStartBlockNumber=$overridingRecoveryStartBlockNumber must be greater than or equal to 1"
    }
    require(lineaBlockGasLimit > 0) {
      "lineaBlockGasLimit=$lineaBlockGasLimit must be greater than 0"
    }
    require(lineaBlockDifficulty >= 0) {
      "lineaBlockDifficulty=$lineaBlockDifficulty must be greater than or equal to 0"
    }

    return PluginConfig(
      lineaSequencerBeneficiaryAddress = lineaSequencerBeneficiaryAddress,
      lineaBlockGasLimit = lineaBlockGasLimit.toULong(),
      lineaBlockDifficulty = lineaBlockDifficulty.toULong(),
      l1SmartContractAddress = l1SmartContractAddress,
      l1Endpoint = l1RpcEndpoint,
      l1PollingInterval = l1PollingInterval.toKotlinDuration(),
      l1GetLogsChunkSize = l1GetLogsChunkSize.toUInt(),
      l1EarliestSearchBlock = l1EarliestSearchBlock,
      l1HighestSearchBlock = l1HighestSearchBlock,
      l1RequestSuccessBackoffDelay = l1RequestSuccessBackoffDelay?.toKotlinDuration() ?: 1.milliseconds,
      l1RequestRetryConfig = RetryConfig(
        backoffDelay = l1RequestRetryBackoffDelay.toKotlinDuration(),
        timeout = l1RequestRetryTimeout?.toKotlinDuration(),
        maxRetries = l1RequestRetryLimit?.toUInt()
      ),
      blobscanEndpoint = blobscanEndpoint,
      blobScanRequestRetryConfig = RetryConfig(
        backoffDelay = blobscanRequestRetryBackoffDelay.toKotlinDuration(),
        timeout = blobscanRequestRetryTimeout?.toKotlinDuration(),
        maxRetries = blobscanRequestRetryLimit?.toUInt()
      ),
      shomeiEndpoint = shomeiEndpoint,
      overridingRecoveryStartBlockNumber = overridingRecoveryStartBlockNumber?.toULong(),
      debugForceSyncStopBlockNumber = debugForceSyncStopBlockNumber?.toULong()
    )
  }

  class AddressConverter : CommandLine.ITypeConverter<Address> {
    override fun convert(value: String): Address {
      return Address.fromHexStringStrict(value) ?: throw CommandLine.TypeConversionException(
        "Invalid address: $value"
      )
    }
  }

  class BlockParameterConverter : CommandLine.ITypeConverter<BlockParameter> {
    override fun convert(value: String): BlockParameter {
      return BlockParameter.parse(value)
    }
  }
}
