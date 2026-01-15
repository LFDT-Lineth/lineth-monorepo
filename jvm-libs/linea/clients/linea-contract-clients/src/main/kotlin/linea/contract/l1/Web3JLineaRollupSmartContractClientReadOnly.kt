package linea.contract.l1

import build.linea.contract.LineaRollupV6
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
import org.web3j.tx.exceptions.ContractCallException
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

  protected fun contractClientAtBlock(blockParameter: BlockParameter): LineaRollupV6 {
    return contractClientAtBlock(blockParameter, LineaRollupV6::class.java)
  }

  protected fun <T : Contract> contractClientAtBlock(blockParameter: BlockParameter, contract: Class<T>): T {
    @Suppress("UNCHECKED_CAST")
    return when {
      LineaRollupV6::class.java.isAssignableFrom(contract) -> LineaRollupV6.load(
        contractAddress,
        web3j,
        fakeCredentials,
        StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
      ).apply {
        this.setDefaultBlockParameter(blockParameter.toWeb3j())
      }
      else -> throw IllegalArgumentException("Unsupported contract type: ${contract::class.java}")
    } as T
  }

  private val smartContractVersionCache = AtomicReference<LineaRollupContractVersion>(null)

  private fun getSmartContractVersion(): SafeFuture<LineaRollupContractVersion> {
    return if (smartContractVersionCache.get() == LineaRollupContractVersion.V7) {
      // once upgraded, it's not downgraded
      SafeFuture.completedFuture(LineaRollupContractVersion.V7)
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

  private fun fetchSmartContractVersion(): SafeFuture<LineaRollupContractVersion> {
    return contractClientAtBlock(BlockParameter.Tag.LATEST, LineaRollupV6::class.java)
      .CONTRACT_VERSION()
      .sendAsync()
      .toSafeFuture()
      .thenApply { version ->
        when {
          version.startsWith("6") -> LineaRollupContractVersion.V6
          version.startsWith("7") -> LineaRollupContractVersion.V7
          else -> throw IllegalStateException("Unsupported contract version: $version")
        }
      }
      .exceptionallyCompose { error ->
        if (error.cause is ContractCallException) {
          // means that contract does not have CONTRACT_VERSION method available yet
          // so it is still V5, so defaulting to V5
          SafeFuture.completedFuture(LineaRollupContractVersion.V6)
        } else {
          SafeFuture.failedFuture(error)
        }
      }
  }

  override fun getAddress(): String = contractAddress

  override fun getVersion(): SafeFuture<LineaRollupContractVersion> = getSmartContractVersion()

  override fun finalizedL2BlockNumber(blockParameter: BlockParameter): SafeFuture<ULong> {
    return contractClientAtBlock(blockParameter)
      .currentL2BlockNumber().sendAsync()
      .thenApply { it.toULong() }
      .toSafeFuture()
  }

  override fun getMessageRollingHash(blockParameter: BlockParameter, messageNumber: Long): SafeFuture<ByteArray> {
    require(messageNumber >= 0) { "messageNumber must be greater than or equal to 0" }

    return contractClientAtBlock(blockParameter).rollingHashes(messageNumber.toBigInteger()).sendAsync().toSafeFuture()
  }

  override fun isBlobShnarfPresent(blockParameter: BlockParameter, shnarf: ByteArray): SafeFuture<Boolean> {
    return getVersion()
      .thenCompose { version ->
        when (version!!) {
          LineaRollupContractVersion.V6,
          LineaRollupContractVersion.V7,
          -> contractClientAtBlock(
            blockParameter,
            LineaRollupV6::class.java,
          ).blobShnarfExists(
            shnarf,
          )
        }
          .sendAsync()
          .thenApply { it != BigInteger.ZERO }
          .toSafeFuture()
      }
  }

  override fun blockStateRootHash(blockParameter: BlockParameter, lineaL2BlockNumber: ULong): SafeFuture<ByteArray> {
    return contractClientAtBlock(blockParameter)
      .stateRootHashes(lineaL2BlockNumber.toBigInteger()).sendAsync()
      .toSafeFuture()
  }
}
