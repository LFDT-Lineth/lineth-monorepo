package linea.coordinator.app

import io.vertx.core.Vertx
import io.vertx.core.http.HttpVersion
import io.vertx.core.http.PoolOptions
import io.vertx.core.net.PfxOptions
import io.vertx.ext.web.client.WebClientOptions
import linea.coordinator.config.v2.SignerConfig
import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import linea.crypto.Web3SignerRestClient
import linea.kotlin.encodeHex
import net.consensys.linea.httprest.client.VertxHttpRestClient
import org.apache.logging.log4j.LogManager
import org.web3j.crypto.Credentials
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.io.FileInputStream
import java.nio.file.Path
import java.security.KeyStore
import javax.net.ssl.KeyManagerFactory
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory

/**
 * Creates a digest signer from coordinator configuration. The returned signer is independent of
 * web3j transaction types, so downstream distributions can add backends without replacing the
 * coordinator's transaction encoding path.
 */
fun interface SignerFactory {
  fun create(
    vertx: Vertx,
    signerConfig: SignerConfig,
  ): Signer<Secp256k1Signature>
}

/** Creates the OSS local-key and Web3Signer backends. */
object DefaultSignerFactory : SignerFactory {
  override fun create(
    vertx: Vertx,
    signerConfig: SignerConfig,
  ): Signer<Secp256k1Signature> =
    when (signerConfig.type) {
      SignerConfig.SignerType.WEB3J ->
        LocalKeySigner(Credentials.create(signerConfig.web3j!!.privateKey.encodeHex()))

      SignerConfig.SignerType.WEB3SIGNER -> createWeb3Signer(vertx, signerConfig.web3signer!!)

      SignerConfig.SignerType.CUSTOM ->
        error("No signer factory is configured for custom signer '${signerConfig.custom!!.name}'")
    }
}

private class LocalKeySigner(
  credentials: Credentials,
) : Signer<Secp256k1Signature> {
  private val keyPair = credentials.ecKeyPair
  private val publicKey = Numeric.toBytesPadded(keyPair.publicKey, PUBLIC_KEY_SIZE_BYTES)

  override fun publicKey(): ByteArray = publicKey.copyOf()

  override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
    require(bytes.size == DIGEST_SIZE_BYTES) {
      "Local signer requires a $DIGEST_SIZE_BYTES-byte digest, but received ${bytes.size} bytes"
    }
    val signature = keyPair.sign(bytes)
    return SafeFuture.completedFuture(Secp256k1Signature(signature.r, signature.s))
  }

  companion object {
    private const val DIGEST_SIZE_BYTES = 32
    private const val PUBLIC_KEY_SIZE_BYTES = 64
  }
}

private fun createWeb3Signer(
  vertx: Vertx,
  config: SignerConfig.Web3SignerConfig,
): Signer<Secp256k1Signature> {
  val endpoint = config.endpoint
  val webClientOptions =
    WebClientOptions()
      .setKeepAlive(config.keepAlive)
      .setProtocolVersion(HttpVersion.HTTP_1_1)
      .setDefaultHost(endpoint.host)
      .setDefaultPort(endpoint.port)
      .also {
        config.tls?.let { tls ->
          loadKeyAndTrustStoreFromFiles(
            webClientOptions = it,
            clientKeystorePath = tls.keyStorePath,
            clientKeystorePassword = tls.keyStorePassword.value,
            trustStorePath = tls.trustStorePath,
            trustStorePassword = tls.trustStorePassword.value,
          )
        }
      }
  val poolOptions = PoolOptions().setHttp1MaxSize(config.maxPoolSize)
  return Web3SignerRestClient(
    VertxHttpRestClient(
      webClientOptions,
      poolOptions,
      vertx,
      LogManager.getLogger("clients.web3signer"),
    ),
    config.publicKey,
  )
}

private fun loadKeyAndTrustStoreFromFiles(
  webClientOptions: WebClientOptions,
  clientKeystorePath: Path,
  clientKeystorePassword: String,
  trustStorePath: Path,
  trustStorePassword: String,
): WebClientOptions {
  val keyStore = KeyStore.getInstance("PKCS12")
  FileInputStream(clientKeystorePath.toAbsolutePath().toString()).use { input ->
    keyStore.load(input, clientKeystorePassword.toCharArray())
  }
  val keyManagerFactory = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm())
  keyManagerFactory.init(keyStore, clientKeystorePassword.toCharArray())

  val trustStore = KeyStore.getInstance("PKCS12")
  FileInputStream(trustStorePath.toAbsolutePath().toString()).use { input ->
    trustStore.load(input, trustStorePassword.toCharArray())
  }
  val trustManagerFactory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
  trustManagerFactory.init(trustStore)

  SSLContext.getInstance("TLS")
    .init(keyManagerFactory.keyManagers, trustManagerFactory.trustManagers, null)

  return webClientOptions
    .setSsl(true)
    .setTrustAll(false)
    .setKeyCertOptions(
      PfxOptions()
        .setPath(clientKeystorePath.toAbsolutePath().toString())
        .setPassword(clientKeystorePassword),
    )
    .setTrustOptions(
      PfxOptions()
        .setPath(trustStorePath.toAbsolutePath().toString())
        .setPassword(trustStorePassword),
    )
    .setVerifyHost(true)
}
