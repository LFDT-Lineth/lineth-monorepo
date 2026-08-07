package linea.coordinator.config.v2.toml

import com.sksamuel.hoplite.Masked
import linea.coordinator.config.v2.SignerConfig
import lineth.config.docs.ConfigDoc
import lineth.config.docs.ConfigSection
import lineth.kotlin.decodeHex
import java.net.URL
import java.nio.file.Path

data class SignerConfigToml(
  @param:ConfigDoc(
    description = "Signer backend to use: WEB3J, WEB3SIGNER, or CUSTOM.",
    example = "web3signer",
  )
  val type: SignerType,
  @param:ConfigSection("Local Web3j signer settings; required when type is WEB3J.")
  val web3j: Web3jConfig?,
  @param:ConfigSection("Remote Web3Signer settings; required when type is WEB3SIGNER.")
  val web3signer: Web3SignerConfig?,
  @param:ConfigSection("Named signer settings; required when type is CUSTOM.")
  val custom: CustomConfig? = null,
) {
  init {
    when {
      type == SignerType.WEB3J && web3j == null -> {
        throw IllegalArgumentException("signetType=$type requires web3j config")
      }

      type == SignerType.WEB3SIGNER && web3signer == null -> {
        throw IllegalArgumentException("signetType=$type requires web3signer config")
      }

      type == SignerType.CUSTOM && custom == null -> {
        throw IllegalArgumentException("signerType=$type requires custom config")
      }
    }
  }

  enum class SignerType(val displayName: String) {
    WEB3J("web3j"),
    WEB3SIGNER("web3signer"),
    CUSTOM("custom"),
    ;

    companion object {
      fun valueOfIgnoreCase(name: String): SignerType {
        return SignerType.entries.firstOrNull { it.displayName.equals(name, ignoreCase = true) }
          ?: throw IllegalArgumentException("Unknown signer type: $name")
      }
    }

    fun reified(): SignerConfig.SignerType {
      return when (this) {
        WEB3J -> SignerConfig.SignerType.WEB3J
        WEB3SIGNER -> SignerConfig.SignerType.WEB3SIGNER
        CUSTOM -> SignerConfig.SignerType.CUSTOM
      }
    }
  }

  data class CustomConfig(
    @param:ConfigDoc("Logical signer name resolved by the injected signer factory.")
    val name: String,
  ) {
    init {
      require(name.isNotBlank()) { "custom signer name must not be blank" }
    }

    fun reified(): SignerConfig.CustomConfig = SignerConfig.CustomConfig(name)
  }

  data class Web3jConfig(
    @param:ConfigDoc("Hex-encoded 32-byte private key used to sign transactions. Masked in logs.")
    val privateKey: Masked,
  ) {
    init {
      runCatching {
        privateKey.value.decodeHex()
      }.onFailure { throw IllegalArgumentException("Invalid hexadecimal encoding of privateKey") }
        .onSuccess { require(it.size == 32) { "privateKey must be 32 bytes (64 hex characters)" } }
    }

    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as Web3jConfig

      return privateKey.value.decodeHex().contentEquals(other.privateKey.value.decodeHex())
    }

    override fun hashCode(): Int {
      return privateKey.hashCode()
    }
  }

  data class Web3SignerConfig(
    @param:ConfigDoc(
      description = "Web3Signer HTTP endpoint.",
      example = "http://web3signer:9000",
    )
    val endpoint: URL,
    @param:ConfigDoc("Hex-encoded 64-byte public key whose corresponding key Web3Signer holds.")
    val publicKey: ByteArray,
    @param:ConfigDoc(description = "Maximum size of the HTTP connection pool to Web3Signer.", default = "10")
    val maxPoolSize: Int = 10,
    @param:ConfigDoc(description = "Whether to keep Web3Signer HTTP connections alive.", default = "true")
    val keepAlive: Boolean = true,
    @param:ConfigSection("TLS settings for the Web3Signer connection; omit for plaintext.")
    val tls: TlsConfig?,
  ) {
    init {
      require(publicKey.size == 64) { "publicKey must be 64 bytes (128 hex characters)" }
      require(maxPoolSize > 0) { "maxPoolSize must be greater than 0" }
    }

    data class TlsConfig(
      @param:ConfigDoc(
        description = "Path to the client keystore used for mutual TLS with Web3Signer.",
        example = "/etc/coordinator/keystore.p12",
      )
      val keyStorePath: Path,
      @param:ConfigDoc("Password for the client keystore. Masked in logs.")
      val keyStorePassword: Masked,
      @param:ConfigDoc(
        description = "Path to the truststore used to validate the Web3Signer certificate.",
        example = "/etc/coordinator/truststore.p12",
      )
      val trustStorePath: Path,
      @param:ConfigDoc("Password for the truststore. Masked in logs.")
      val trustStorePassword: Masked,
    ) {
      init {
        require(!keyStorePassword.value.isEmpty()) { "keyStorePassword must not be empty" }
        require(!trustStorePassword.value.isEmpty()) { "trustStorePassword must not be empty" }
        require(!keyStorePath.toString().isEmpty()) { "keyStorePath must not be empty" }
        require(!trustStorePath.toString().isEmpty()) { "trustStorePath must not be empty" }
      }

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
      if (tls != other.tls) return false

      return true
    }

    override fun hashCode(): Int {
      var result = maxPoolSize
      result = 31 * result + keepAlive.hashCode()
      result = 31 * result + endpoint.hashCode()
      result = 31 * result + publicKey.contentHashCode()
      result = 31 * result + (tls?.hashCode() ?: 0)
      return result
    }
  }

  fun reified(): SignerConfig {
    return SignerConfig(
      type = type.reified(),
      web3j = web3j?.let { SignerConfig.Web3jConfig(it.privateKey.value.decodeHex()) },
      web3signer =
      web3signer?.let {
        SignerConfig.Web3SignerConfig(
          endpoint = it.endpoint,
          publicKey = it.publicKey,
          maxPoolSize = it.maxPoolSize,
          keepAlive = it.keepAlive,
          tls =
          it.tls?.let { tls ->
            SignerConfig.Web3SignerConfig.TlsConfig(
              keyStorePath = tls.keyStorePath,
              keyStorePassword = tls.keyStorePassword,
              trustStorePath = tls.trustStorePath,
              trustStorePassword = tls.trustStorePassword,
            )
          },
        )
      },
      custom = custom?.reified(),
    )
  }
}
