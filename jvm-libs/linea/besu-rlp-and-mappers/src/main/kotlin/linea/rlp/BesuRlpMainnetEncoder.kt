package linea.rlp

import io.vertx.core.Vertx
import net.consensys.linea.async.toSafeFuture
import org.hyperledger.besu.ethereum.core.Block
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.Callable

object BesuMainnetBlockRlpEncoder : BesuBlockRlpEncoder {
  override fun encode(block: Block): ByteArray = RLP.encodeBlock(block)
}

object BesuMainnetBlockRlpDecoder : BesuBlockRlpDecoder {
  override fun decode(block: ByteArray): Block = RLP.decodeBlockWithMainnetFunctions(block)
}

class BesuRlpMainnetEncoderAsyncVertxImpl(
  val vertx: Vertx,
  val encoder: BesuBlockRlpEncoder = BesuMainnetBlockRlpEncoder
) : BesuBlockRlpEncoderAsync {
  override fun encodeAsync(block: Block): SafeFuture<ByteArray> {
    return vertx.executeBlocking(
      Callable {
        encoder.encode(block)
      },
      false
    )
      .toSafeFuture()
  }
}

/**
 * We can decode with Mainnet full functionality or
 * with custom decoder for blob decompressed transactions without signature and blocks without header
 * used for state reconstruction
 */
class BesuRlpDecoderAsyncVertxImpl(
  private val vertx: Vertx,
  private val decoder: BesuBlockRlpDecoder
) : BesuBlockRlpDecoderAsync {
  companion object {
    fun mainnetDecoder(vertx: Vertx): BesuBlockRlpDecoderAsync {
      return BesuRlpDecoderAsyncVertxImpl(vertx, BesuMainnetBlockRlpDecoder)
    }

    fun blobDecoder(vertx: Vertx): BesuBlockRlpDecoderAsync {
      return BesuRlpDecoderAsyncVertxImpl(vertx, BesuRlpBlobDecoder)
    }
  }

  override fun decodeAsync(block: ByteArray): SafeFuture<Block> {
    return vertx.executeBlocking(
      Callable {
        decoder.decode(block)
      },
      false
    )
      .toSafeFuture()
  }
}
