package lineth.crypto

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.map
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.impl.HttpResponseImpl
import lineth.httprest.client.HttpRestClient
import lineth.kotlin.decodeHex
import lineth.kotlin.encodeHex
import tech.pegasys.teku.infrastructure.async.SafeFuture

class Web3SignerRestClient(
  private val client: HttpRestClient,
  private val publicKey: ByteArray,
) : Signer<Secp256k1Signature> {
  private val publicKeyHex = publicKey.encodeHex()

  override fun publicKey(): ByteArray = publicKey

  override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
    require(bytes.size == DIGEST_SIZE_BYTES) {
      "Web3Signer requires a $DIGEST_SIZE_BYTES-byte digest, but received ${bytes.size} bytes"
    }
    val path = WEB3SIGNER_SIGN_ENDPOINT + publicKeyHex
    val requestJson =
      """
      {"data":"${bytes.encodeHex()}","applyHash":false}
      """.trimIndent()
    val buffer = Buffer.buffer(requestJson)

    return client.post(path, buffer).thenApply { response ->
      when (val body = response.map { (it as HttpResponseImpl<*>).body().toString() }) {
        is Ok -> {
          val signature = body.value.decodeHex()
          require(signature.size == WEB3SIGNER_SIGNATURE_SIZE_BYTES) {
            "Web3Signer returned a ${signature.size}-byte signature; expected " +
              "$WEB3SIGNER_SIGNATURE_SIZE_BYTES bytes (r || s || v)"
          }
          Secp256k1Signature.fromRSBytes(signature.sliceArray(0 until Secp256k1Signature.SIZE_BYTES))
        }

        is Err -> throw body.error.asException()
      }
    }
  }

  companion object {
    const val WEB3SIGNER_SIGN_ENDPOINT = "/api/v1/eth1/sign/"
    private const val DIGEST_SIZE_BYTES = 32
    private const val WEB3SIGNER_SIGNATURE_SIZE_BYTES = Secp256k1Signature.SIZE_BYTES + 1
  }
}
