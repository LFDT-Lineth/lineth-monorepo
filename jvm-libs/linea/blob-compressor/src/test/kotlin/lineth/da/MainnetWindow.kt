package lineth.da

import io.vertx.core.json.JsonArray
import io.vertx.core.json.JsonObject
import org.junit.jupiter.api.Assumptions.assumeTrue
import java.nio.file.Files
import java.nio.file.Path
import java.util.HexFormat
import java.util.zip.GZIPInputStream

/**
 * The consecutive Linea mainnet blocks of the gzipped execution-spec blockchain test that the `lineth.da.testBlocks`
 * property names: the only source of block RLPs for the tests, with the chain ID of the file.
 */
internal object MainnetWindow {
  private val path: Path? = System.getProperty("lineth.da.testBlocks")?.takeIf { it.isNotBlank() }?.let { Path.of(it) }

  private val case: JsonObject by lazy {
    val json = GZIPInputStream(Files.newInputStream(path!!)).use { it.readAllBytes().toString(Charsets.UTF_8) }
    JsonObject(json).let { file -> file.getJsonObject(file.fieldNames().single()) }
  }

  private val window: JsonArray by lazy { case.getJsonArray("blocks") }

  val chainId: ULong by lazy { case.getJsonObject("config").getString("chainid").hexToULong() }

  val size: Int get() = window.size()

  fun assumeAvailable() {
    assumeTrue(path != null, "the lineth.da.testBlocks property names no file")
    assumeTrue(Files.isRegularFile(path!!), "the lineth.da.testBlocks file $path does not exist")
  }

  /** The full block RLPs of [count] consecutive blocks from the one at index [from] of the window. */
  fun blocks(from: Int, count: Int): List<ByteArray> = (from until from + count).map(::rlp)

  fun blockNumber(index: Int): ULong = block(index).getJsonObject("blockHeader").getString("number").hexToULong()

  /** The first block that holds a legacy transaction signed under the chain ID of the file (EIP-155). */
  fun blockWithLegacyTransaction(): ByteArray = rlp((0 until size).first { holdsProtectedLegacyTransaction(block(it)) })

  private fun block(index: Int): JsonObject = window.getJsonObject(index)

  private fun rlp(index: Int): ByteArray = HexFormat.of().parseHex(block(index).getString("rlp").removePrefix("0x"))

  private fun holdsProtectedLegacyTransaction(block: JsonObject): Boolean {
    val transactions = block.getJsonArray("transactions")
    return (0 until transactions.size()).map(transactions::getJsonObject).any { transaction ->
      val legacy = (transaction.getString("type") ?: "0x00").hexToULong() == 0UL
      legacy && transaction.getString("v").hexToULong() >= 35UL
    }
  }

  private fun String.hexToULong(): ULong = removePrefix("0x").toULong(16)
}
