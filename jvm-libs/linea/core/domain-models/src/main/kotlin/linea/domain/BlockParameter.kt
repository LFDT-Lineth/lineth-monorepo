package linea.domain

import linea.kotlin.decodeHex
import linea.kotlin.encodeHex

sealed interface BlockParameter {
  fun getTag(): String

  fun getNumber(): ULong

  companion object {
    private const val BLOCK_HASH_HEX_LENGTH = 64

    fun fromNumber(blockNumber: Number): BlockNumber {
      require(blockNumber.toLong() >= 0) { "block number must be greater or equal than 0, value=$blockNumber" }
      return BlockNumber(blockNumber.toLong().toULong())
    }

    fun fromNumber(blockNumber: ULong): BlockNumber = BlockNumber(blockNumber)

    fun fromHash(blockHash: ByteArray): BlockHash = BlockHash(blockHash)

    fun fromHash(blockHashHex: String): BlockHash = BlockHash(blockHashHex.decodeHex())

    fun parse(input: String): BlockParameter {
      return try {
        Tag.fromString(input)
      } catch (e: IllegalArgumentException) {
        val normalized = input.lowercase()
        if (normalized.startsWith("0x")) {
          val hexBody = normalized.drop(2)
          if (hexBody.length == BLOCK_HASH_HEX_LENGTH) {
            return BlockHash("0x$hexBody".decodeHex())
          }
          val blockNumber = hexBody.toULongOrNull(radix = 16)
            ?: throw IllegalArgumentException("Invalid BlockParameter: $input")
          return BlockNumber(blockNumber)
        }
        val blockNumber = input.toULongOrNull(radix = 10)
          ?: throw IllegalArgumentException("Invalid BlockParameter: $input")
        blockNumber.toBlockParameter()
      }
    }

    // Handy extensions
    fun Number.toBlockParameter(): BlockParameter = fromNumber(this)
    fun UInt.toBlockParameter(): BlockParameter = BlockNumber(this.toULong())
    fun ULong.toBlockParameter(): BlockParameter = BlockNumber(this)
  }

  enum class Tag(val value: String) : BlockParameter {
    PENDING("pending"),
    LATEST("latest"),
    EARLIEST("earliest"),
    SAFE("safe"),
    FINALIZED("finalized"),
    ;

    override fun getTag(): String = value
    override fun getNumber(): ULong = throw UnsupportedOperationException(
      "getNumber isn't supposed to be called on a block tag!",
    )

    companion object {
      @JvmStatic
      fun fromString(value: String): Tag = kotlin.runCatching { Tag.valueOf(value.uppercase()) }
        .getOrElse {
          throw IllegalArgumentException(
            "BlockParameter Tag=$value is invalid. Valid values: ${Tag.entries.joinToString(", ")}",
          )
        }
    }
  }

  @JvmInline
  value class BlockNumber(private val parameter: ULong) : BlockParameter {

    override fun getTag(): String {
      throw UnsupportedOperationException("getTag isn't supported on BlockNumber!")
    }

    override fun getNumber(): ULong {
      return parameter
    }

    override fun toString(): String {
      return parameter.toString()
    }
  }

  data class BlockHash(private val hash: ByteArray) : BlockParameter {
    init {
      require(hash.size == 32) { "block hash must be 32 bytes, got ${hash.size}" }
    }

    fun getHash(): ByteArray = hash

    override fun getTag(): String {
      throw UnsupportedOperationException("getTag isn't supported on BlockHash!")
    }

    override fun getNumber(): ULong {
      throw UnsupportedOperationException("getNumber isn't supported on BlockHash!")
    }

    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (other !is BlockHash) return false
      return hash.contentEquals(other.hash)
    }

    override fun hashCode(): Int = hash.contentHashCode()

    override fun toString(): String {
      return hash.encodeHex(prefix = true)
    }
  }
}
