package lineth.coordination.riscv.da

import lineth.da.DaCodecVersion
import lineth.da.DaCompressor
import lineth.da.DaCompressorFactory
import lineth.da.DaStreamErrorType
import lineth.da.DaStreamException
import java.io.ByteArrayOutputStream

/**
 * Pure-Kotlin [DaCompressor] for [FakeDaStream]. Stand-in: no zstd and no block truncation, the frame is the
 * concatenated block RLPs. Stateless and thread-safe.
 */
class FakeDaCompressor : DaCompressor {
  override val version: DaCodecVersion = DaCodecVersion.FAKE

  override fun compress(blockRlps: List<ByteArray>): ByteArray {
    if (blockRlps.isEmpty()) {
      throw DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "a conflation must have at least one block")
    }
    return ByteArrayOutputStream().also { out -> blockRlps.forEach(out::writeBytes) }.toByteArray()
  }

  override fun close() = Unit
}

/** Creates [FakeDaCompressor]s. The chain ID is ignored: the fake neither compresses nor truncates. */
class FakeDaCompressorFactory : DaCompressorFactory {
  override fun create(version: DaCodecVersion, chainId: ULong): DaCompressor {
    if (version != DaCodecVersion.FAKE) {
      throw DaStreamException(
        DaStreamErrorType.INVALID_ARGUMENT,
        "FakeDaCompressorFactory only creates ${DaCodecVersion.FAKE}, got $version",
      )
    }
    return FakeDaCompressor()
  }
}
