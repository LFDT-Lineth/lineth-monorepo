package linea.contract.l1

import build.linea.contract.ValidiumV1
import linea.domain.BlockParameter
import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import linea.web3j.domain.toWeb3j
import net.consensys.linea.async.toSafeFuture
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.tx.gas.StaticGasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

private val fakeCredentials = Credentials.create(ByteArray(32).encodeHex())

open class Web3JLineaValidiumSmartContractClientReadOnly(
  val web3j: Web3j,
  val contractAddress: String,
) : LineaValidiumSmartContractClientReadOnly {
  protected fun contractClientAtBlock(blockParameter: BlockParameter): ValidiumV1 {
    return ValidiumV1.load(
      contractAddress,
      web3j,
      fakeCredentials,
      StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
    ).apply {
      this.setDefaultBlockParameter(blockParameter.toWeb3j())
    }
  }

  override fun getAddress(): String = contractAddress

  override fun getVersion(blockParameter: BlockParameter): SafeFuture<LineaValidiumContractVersion> =
    SafeFuture.completedFuture(LineaValidiumContractVersion.V1)

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
    return contractClientAtBlock(blockParameter)
      .blobShnarfExists(shnarf).sendAsync()
      .thenApply { it != BigInteger.ZERO }
      .toSafeFuture()
  }

  override fun blockStateRootHash(blockParameter: BlockParameter, lineaL2BlockNumber: ULong): SafeFuture<ByteArray> {
    return contractClientAtBlock(blockParameter)
      .stateRootHashes(lineaL2BlockNumber.toBigInteger()).sendAsync()
      .toSafeFuture()
  }
}
