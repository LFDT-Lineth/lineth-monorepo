package net.consensys.zkevm.ethereum.crypto

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.map
import io.vertx.core.buffer.Buffer
import io.vertx.ext.web.client.impl.HttpResponseImpl
import net.consensys.linea.httprest.client.HttpRestClient
import org.apache.tuweni.bytes.Bytes
import java.math.BigInteger

class Web3SignerRestClient(private val client: HttpRestClient, private val publicKey: String) :
  Signer {
  override fun sign(bytes: Bytes): Pair<BigInteger, BigInteger> {
    val path = WEB3SIGNER_SIGN_ENDPOINT + publicKey
    val requestJson =
      """
      {"data":"$bytes"}
      """.trimIndent()
    val buffer = Buffer.buffer(requestJson)

    val response =
      client.post(path, buffer).get().map {
        it as HttpResponseImpl<*>
        it.body().toString()
      }

    return when (response) {
      is Ok -> {
        val signature = Bytes.fromHexString(response.value.removePrefix("0x")).toArray()
        val rSize = 32
        val sSize = 32
        val rSlice = signature.sliceArray(0 until rSize)
        val sSlice = signature.sliceArray(rSize until rSize + sSize)
        val r = BigInteger(1, rSlice)
        val s = BigInteger(1, sSlice)
        Pair(r, s)
      }
      is Err -> throw response.error.asException()
    }
  }

  companion object {
    const val WEB3SIGNER_SIGN_ENDPOINT = "/api/v1/eth1/sign/"
  }
}
