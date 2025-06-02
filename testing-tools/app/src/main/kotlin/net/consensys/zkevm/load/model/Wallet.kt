package net.consensys.zkevm.load.model

import org.web3j.crypto.Credentials
import org.web3j.utils.Numeric
import java.math.BigInteger
import java.util.concurrent.atomic.AtomicReference

data class Wallet(
  val privateKey: String,
  val credentials: Credentials,
  val id: Int,
  @Transient val theoreticalNonce: AtomicReference<BigInteger>,
  var initialNonce: BigInteger?,
) {
  val address: String = credentials.address

  constructor(privateKey: String, id: Int, initialNonce: BigInteger?) :
    this(
      privateKey,
      Credentials.create(privateKey),
      id,
      AtomicReference<BigInteger>(initialNonce),
      initialNonce,
    )

  fun encodedAddress(): String {
    return Numeric.prependHexPrefix(address)
  }

  val theoreticalNonceValue: BigInteger
    get() = theoreticalNonce.get()

  fun incrementTheoreticalNonce() {
    theoreticalNonce.set(theoreticalNonce.get()!!.add(BigInteger.ONE))
  }
}
