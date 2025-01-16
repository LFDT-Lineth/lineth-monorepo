package net.consensys.linea.nativecompressor

import linea.rlp.RLP

object CompressorTestData {
  val blocksRlpEncoded: List<ByteArray> = RLP.decodeList(readResourcesFile("blocks_rlp.bin"))

  private fun readResourcesFile(fileName: String): ByteArray {
    return Thread.currentThread().getContextClassLoader().getResourceAsStream(fileName)
      ?.readAllBytes()
      ?: throw IllegalArgumentException("File not found in jar resources: file=$fileName")
  }
}
