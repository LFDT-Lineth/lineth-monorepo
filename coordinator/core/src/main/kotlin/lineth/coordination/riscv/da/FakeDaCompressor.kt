package lineth.coordination.riscv.da

import linea.da.CompressedConflation
import linea.da.DaCodecVersion
import linea.da.DaCompressor
import linea.da.DaCompressorFactory
import linea.da.DaStreamErrorType
import linea.da.DaStreamException
import linea.domain.BlockInterval
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLP
import java.io.ByteArrayOutputStream
import java.nio.file.Path

/**
 * Pure-Kotlin [DaCompressor] for [FakeDaStream]. Stand-in: no zstd and no block truncation, the frame is the
 * concatenated block RLPs. Stateless and thread-safe.
 */
class FakeDaCompressor : DaCompressor {
  override val version: DaCodecVersion = DaCodecVersion.FAKE

  override fun compress(blockRlps: List<ByteArray>): CompressedConflation {
    if (blockRlps.isEmpty()) {
      throw DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "a conflation must have at least one block")
    }
    val blockNumbers = blockRlps.map(::decodeBlockNumber)
    blockNumbers.zipWithNext().forEach { (current, next) ->
      if (next != current + 1UL) {
        throw DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "blocks are not consecutive: $current then $next")
      }
    }
    val body = ByteArrayOutputStream().also { out -> blockRlps.forEach(out::writeBytes) }.toByteArray()
    return CompressedConflation(
      blockInterval = BlockInterval(blockNumbers.first(), blockNumbers.last()),
      compressedData = body,
    )
  }

  override fun close() = Unit

  private companion object {
    /** Header field index of the block number: after parentHash, ommersHash, coinbase, 3 roots, bloom, difficulty. */
    const val HEADER_NUMBER_INDEX = 8

    fun decodeBlockNumber(blockRlp: ByteArray): ULong {
      return try {
        val input = RLP.input(Bytes.wrap(blockRlp))
        input.enterList()
        input.enterList()
        repeat(HEADER_NUMBER_INDEX) { input.skipNext() }
        input.readLongScalar().toULong()
      } catch (e: Exception) {
        throw DaStreamException(DaStreamErrorType.INVALID_BLOCK_RLP, "cannot decode block number: ${e.message}", e)
      }
    }
  }
}

/** Creates [FakeDaCompressor]s. The dictionary is ignored: the fake does not compress. */
class FakeDaCompressorFactory : DaCompressorFactory {
  override fun create(version: DaCodecVersion, dictionary: Path): DaCompressor {
    if (version != DaCodecVersion.FAKE) {
      throw DaStreamException(
        DaStreamErrorType.INVALID_ARGUMENT,
        "FakeDaCompressorFactory only creates ${DaCodecVersion.FAKE}, got $version",
      )
    }
    return FakeDaCompressor()
  }
}
