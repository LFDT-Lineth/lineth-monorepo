package linea.crypto

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.map
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.impl.HttpResponseImpl
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import net.consensys.linea.httprest.client.HttpRestClient
import tech.pegasys.teku.infrastructure.async.SafeFuture

class Web3SignerRestClient(
  private val client: HttpRestClient,
  private val publicKey: ByteArray,
) : Signer {
  private val publicKeyHex = publicKey.encodeHex()

  override fun publicKey(): ByteArray = publicKey

  override fun sign(bytes: ByteArray): SafeFuture<ByteArray> {
    val path = WEB3SIGNER_SIGN_ENDPOINT + publicKeyHex
    val requestJson =
      """
      {"data":"${bytes.encodeHex()}"}
      """.trimIndent()
    val buffer = Buffer.buffer(requestJson)

    return client.post(path, buffer).thenApply { response ->
      when (val body = response.map { (it as HttpResponseImpl<*>).body().toString() }) {
        is Ok -> {
          val signature = body.value.decodeHex()
          Secp256k1Signature.fromBytes(signature.sliceArray(0 until Secp256k1Signature.SIZE_BYTES)).toBytes()
        }

        is Err -> throw body.error.asException()
      }
    }
  }

  companion object {
    const val WEB3SIGNER_SIGN_ENDPOINT = "/api/v1/eth1/sign/"
  }
}
