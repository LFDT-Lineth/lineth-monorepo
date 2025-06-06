package linea.coordinator.config.v2

import java.net.URL

data class SignerConfig(
  val type: SignerType,
  val web3j: Web3jConfig?,
  val web3signer: Web3SignerConfig?,
) {
  init {
    when {
      type == SignerType.WEB3J && web3j == null -> {
        throw IllegalArgumentException("signetType=$type requires web3j config")
      }

      type == SignerType.WEB3SIGNER && web3signer == null -> {
        throw IllegalArgumentException("signetType=$type requires web3signer config")
      }
    }
  }

  enum class SignerType() {
    WEB3J,
    WEB3SIGNER,
  }

  data class Web3jConfig(
    val privateKey: ByteArray,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as Web3jConfig

      return privateKey.contentEquals(other.privateKey)
    }

    override fun hashCode(): Int {
      return privateKey.hashCode()
    }

    override fun toString(): String {
      return "Web3jConfig(privateKey=***${privateKey.size}bytes***)"
    }
  }

  data class Web3SignerConfig(
    val endpoint: URL,
    val publicKey: ByteArray,
    val maxPoolSize: Int = 10,
    val keepAlive: Boolean = true,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as Web3SignerConfig

      if (maxPoolSize != other.maxPoolSize) return false
      if (keepAlive != other.keepAlive) return false
      if (endpoint != other.endpoint) return false
      if (!publicKey.contentEquals(other.publicKey)) return false

      return true
    }

    override fun hashCode(): Int {
      var result = maxPoolSize
      result = 31 * result + keepAlive.hashCode()
      result = 31 * result + endpoint.hashCode()
      result = 31 * result + publicKey.contentHashCode()
      return result
    }
  }
}
