package net.consensys.linea.nativecompressor

import io.vertx.core.json.JsonObject
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.rlp.RLP
import net.consensys.linea.testing.filesystem.getPathTo
import org.hyperledger.besu.ethereum.core.Block
import java.nio.file.Files
import java.nio.file.Path

fun loadBlocksFromProverRequests(
  proverExecutionRequestsFolder: Path,
): List<Pair<Block, ByteArray>> {
  val blocks = Files
    .list(proverExecutionRequestsFolder)
    .toList()
    .map { file ->
      JsonObject(Files.readString(file))
        .getJsonArray("blocksData")
        .map { block ->
          block as JsonObject
          // block RLP encoded
          val rlp = block.getString("rlp").decodeHex()
          RLP.decodeBlockWithMainnetFunctions(rlp) to rlp
        }
    }
    .toList()
    .flatten()
  return blocks
    .sortedBy { it.first.header.number }
    .also {
      it.forEach {
        println("block=${it.first.header} rlp=${it.second.encodeHex()}")
      }
    }
}

fun generateEncodeBlocksToBinaryFromProverRequests(
  proverExecutionRequestsFolder: Path,
  outputFilePath: Path,
) {
  val blocks = loadBlocksFromProverRequests(proverExecutionRequestsFolder)
  Files.write(outputFilePath, RLP.encodeList(blocks.map { it.second }))
}

fun loadBlocksRlpEncoded(
  binFile: Path,
): List<ByteArray> {
  return RLP.decodeList(Files.readAllBytes(binFile))
}

fun main() {
  val proverExecutionRequestsDir = getPathTo("tmp/local/prover/v3/execution/requests-done/")
  val destFile = getPathTo("jvm-libs/linea/blob-compressor/src/testFixtures/resources")
    .resolve("blocks_rlp.bin")

  generateEncodeBlocksToBinaryFromProverRequests(
    proverExecutionRequestsDir,
    destFile,
  )

  // Just a visual indicator that it can read/decode again
  println("\n\n")
  loadBlocksRlpEncoded(destFile)
    .map(RLP::decodeBlockWithMainnetFunctions)
    .forEach {
      println("block=$it")
    }
}
