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
import java.math.BigInteger

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
          val rSize = 32
          val sSize = 32
          val r = BigInteger(1, signature.sliceArray(0 until rSize))
          val s = BigInteger(1, signature.sliceArray(rSize until rSize + sSize))
          Secp256k1Signature(r, s).toBytes()
        }

        is Err -> throw body.error.asException()
      }
    }
  }

  companion object {
    const val WEB3SIGNER_SIGN_ENDPOINT = "/api/v1/eth1/sign/"
  }
}
