package linea.contract.l1

import build.linea.contract.LineaRollupV6
import build.linea.contract.LineaRollupV8
import linea.domain.BlockParameter
import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import linea.web3j.domain.toWeb3j
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.tx.Contract
import org.web3j.tx.gas.StaticGasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.atomic.AtomicReference

private val fakeCredentials = Credentials.create(ByteArray(32).encodeHex())

open class Web3JLineaRollupSmartContractClientReadOnly(
  val web3j: Web3j,
  val contractAddress: String,
  private val log: Logger = LogManager.getLogger(Web3JLineaRollupSmartContractClientReadOnly::class.java),
) : LineaRollupSmartContractClientReadOnly {

  protected fun contractClientV8AtBlock(blockParameter: BlockParameter): LineaRollupV8 {
    return contractClientAtBlock(blockParameter, LineaRollupV8::class.java)
  }

  protected fun <T : Contract> loadContractClient(contract: Class<T>): T {
    @Suppress("UNCHECKED_CAST")
    return when {
      LineaRollupV6::class.java.isAssignableFrom(contract) -> LineaRollupV6.load(
        contractAddress,
        web3j,
        fakeCredentials,
        StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
      )

      LineaRollupV8::class.java.isAssignableFrom(contract) -> LineaRollupV8.load(
        contractAddress,
        web3j,
        fakeCredentials,
        StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
      )

      else -> throw IllegalArgumentException("Unsupported contract type: ${contract::class.java}")
    } as T
  }

  protected fun <T : Contract> contractClientAtBlock(blockParameter: BlockParameter, contract: Class<T>): T {
    @Suppress("UNCHECKED_CAST")
    return loadContractClient(contract).apply {
      this.setDefaultBlockParameter(blockParameter.toWeb3j())
    }
  }

  private val smartContractVersionCache = AtomicReference<LineaRollupContractVersion>(null)

  private fun getSmartContractVersion(): SafeFuture<LineaRollupContractVersion> {
    return if (smartContractVersionCache.get() == LineaRollupContractVersion.V8) {
      // once upgraded, it's not downgraded
      SafeFuture.completedFuture(LineaRollupContractVersion.V8)
    } else {
      fetchSmartContractVersion()
        .thenPeek { contractLatestVersion ->
          if (smartContractVersionCache.get() != null &&
            contractLatestVersion != smartContractVersionCache.get()
          ) {
            log.info(
              "Smart contract upgraded: prevVersion={} upgradedVersion={}",
              smartContractVersionCache.get(),
              contractLatestVersion,
            )
          }
          smartContractVersionCache.set(contractLatestVersion)
        }
    }
  }

  private fun fetchSmartContractVersion(
    blockParameter: BlockParameter = BlockParameter.Tag.LATEST,
  ): SafeFuture<LineaRollupContractVersion> {
    return contractClientAtBlock(blockParameter, LineaRollupV6::class.java)
      .CONTRACT_VERSION()
      .sendAsync()
      .toSafeFuture()
      .thenApply(::parseContractVersion)
  }

  private fun parseContractVersion(version: String): LineaRollupContractVersion {
    return when {
      version.startsWith("6") -> LineaRollupContractVersion.V6
      version.startsWith("7") -> LineaRollupContractVersion.V7
      version.startsWith("8") -> LineaRollupContractVersion.V8
      else -> throw IllegalStateException("Unsupported contract version: $version")
    }
  }

  override fun getAddress(): String = contractAddress

  override fun getVersion(blockParameter: BlockParameter): SafeFuture<LineaRollupContractVersion> {
    return if (blockParameter == BlockParameter.Tag.LATEST) {
      getSmartContractVersion()
    } else {
      fetchSmartContractVersion(blockParameter)
    }
  }

  override fun finalizedL2BlockNumber(blockParameter: BlockParameter): SafeFuture<ULong> {
    return contractClientV8AtBlock(blockParameter)
      .currentL2BlockNumber().sendAsync()
      .thenApply { it.toULong() }
      .toSafeFuture()
  }

  override fun getMessageRollingHash(blockParameter: BlockParameter, messageNumber: Long): SafeFuture<ByteArray> {
    require(messageNumber >= 0) { "messageNumber must be greater than or equal to 0" }

    return contractClientV8AtBlock(
      blockParameter,
    ).rollingHashes(messageNumber.toBigInteger()).sendAsync().toSafeFuture()
  }

  override fun isBlobShnarfPresent(blockParameter: BlockParameter, shnarf: ByteArray): SafeFuture<Boolean> {
    return getVersion()
      .thenCompose { version ->
        when (version) {
          LineaRollupContractVersion.V6,
          LineaRollupContractVersion.V7,
          LineaRollupContractVersion.V8,
          -> contractClientV8AtBlock(blockParameter).blobShnarfExists(shnarf)
        }
          .sendAsync()
          .thenApply { it != BigInteger.ZERO }
          .toSafeFuture()
      }
  }

  override fun blockStateRootHash(blockParameter: BlockParameter, lineaL2BlockNumber: ULong): SafeFuture<ByteArray> {
    return contractClientV8AtBlock(blockParameter)
      .stateRootHashes(lineaL2BlockNumber.toBigInteger()).sendAsync()
      .toSafeFuture()
  }
}
