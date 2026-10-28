package lineth.coordinator.config.v2

import com.sksamuel.hoplite.Masked
import linea.kotlin.encodeHex
import java.net.URL
import java.nio.file.Path
import kotlin.io.path.exists

data class SignerConfig(
  val type: SignerType,
  val web3j: Web3jConfig?,
  val web3signer: Web3SignerConfig?,
  val custom: CustomConfig? = null,
  /** Backend settings of a registered signer type (`type` is none of the built-in ones). */
  val registered: RegisteredConfig? = null,
) {
  init {
    when (type) {
      SignerType.WEB3J -> {
        require(web3j != null) {
          "signerType=$type requires web3j config"
        }
      }

      SignerType.WEB3SIGNER -> {
        require(web3signer != null) {
          "signerType=$type requires web3signer config"
        }
        if (web3signer.tls != null) {
          require(web3signer.endpoint.protocol == "https") {
            "signerType=$type with TLS configs requires web3signer.endpoint in https"
          }
          require(!web3signer.tls.keyStorePassword.value.isEmpty()) {
            "web3signer.tls.keyStorePassword must not be empty"
          }
          require(!web3signer.tls.trustStorePassword.value.isEmpty()) {
            "web3signer.tls.trustStorePassword must not be empty"
          }
          require(web3signer.tls.keyStorePath.exists()) {
            "web3signer.tls.keyStorePath=${web3signer.tls.keyStorePath} must point to an existing file"
          }
          require(web3signer.tls.trustStorePath.exists()) {
            "web3signer.tls.trustStorePath=${web3signer.tls.trustStorePath} must point to an existing file"
          }
        }
      }

      SignerType.CUSTOM -> {
        requireNotNull(custom) {
          "signerType=$type requires custom config"
        }
      }

      else -> {
        requireNotNull(registered) {
          "signerType=$type requires registered config"
        }
        require(registered.type == type) {
          "registered config type=${registered.type} must match signerType=$type"
        }
      }
    }
  }

  /**
   * Built-in types are [WEB3J], [WEB3SIGNER] and the deprecated [CUSTOM]; any other name is a type registered
   * by a custom signer factory, configured in the `[..signer.<type>]` table. Names are case-insensitive.
   */
  class SignerType(name: String) {
    val name: String = name.trim().lowercase()

    init {
      require(this.name.isNotEmpty()) { "signer type must not be blank" }
    }

    val isBuiltIn: Boolean get() = this in BUILT_IN

    override fun equals(other: Any?): Boolean = other is SignerType && other.name == name

    override fun hashCode(): Int = name.hashCode()

    override fun toString(): String = name.uppercase()

    companion object {
      val WEB3J = SignerType("web3j")
      val WEB3SIGNER = SignerType("web3signer")
      val CUSTOM = SignerType("custom")
      private val BUILT_IN = setOf(WEB3J, WEB3SIGNER, CUSTOM)
    }
  }

  /** Settings of a registered signer type: the flat key/value pairs of the `[..signer.<type>]` table. */
  data class RegisteredConfig(val type: SignerType, val settings: Map<String, Masked>)

  data class CustomConfig(val name: String) {
    init {
      require(name.isNotBlank()) { "custom signer name must not be blank" }
    }
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
    val tls: TlsConfig?,
  ) {
    data class TlsConfig(
      val keyStorePath: Path,
      val keyStorePassword: Masked,
      val trustStorePath: Path,
      val trustStorePassword: Masked,
    ) {
      override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as TlsConfig

        if (keyStorePath != other.keyStorePath) return false
        if (keyStorePassword != other.keyStorePassword) return false
        if (trustStorePath != other.trustStorePath) return false
        if (trustStorePassword != other.trustStorePassword) return false

        return true
      }

      override fun hashCode(): Int {
        var result = keyStorePath.hashCode()
        result = 31 * result + keyStorePassword.hashCode()
        result = 31 * result + trustStorePath.hashCode()
        result = 31 * result + trustStorePassword.hashCode()
        return result
      }
    }

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

    override fun toString(): String {
      return "Web3SignerConfig(" +
        "endpoint=$endpoint, " +
        "publicKey=${publicKey.encodeHex()}, " +
        "maxPoolSize=$maxPoolSize," +
        " keepAlive=$keepAlive, " +
        "tls=$tls)"
    }
  }
}
