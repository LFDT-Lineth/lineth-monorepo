package linea.domain

import linea.kotlin.decodeHex
import linea.kotlin.encodeHex

sealed interface BlockParameter {
  companion object {
    private const val BLOCK_HASH_HEX_LENGTH = 64

    fun fromNumber(blockNumber: Number): BlockNumber {
      require(blockNumber.toLong() >= 0) { "block number must be greater or equal than 0, value=$blockNumber" }
      return BlockNumber(blockNumber.toLong().toULong())
    }

    fun fromNumber(blockNumber: ULong): BlockNumber = BlockNumber(blockNumber)

    fun fromHash(blockHash: ByteArray): BlockHash = BlockHash(blockHash.copyOf())

    fun fromHash(blockHashHex: String): BlockHash = BlockHash(blockHashHex.decodeHex())

    fun parse(input: String): BlockParameter {
      return try {
        // Try to parse the input as a tag
        Tag.fromString(input)
      } catch (e: IllegalArgumentException) {
        // If it's not a valid tag, try to parse it as a block hash or block number
        if (input.startsWith("0x")) {
          val hexBody = input.drop(2)
          if (hexBody.length == BLOCK_HASH_HEX_LENGTH) {
            return fromHash(input)
          }
          (
            hexBody.toULongOrNull(radix = 16)
              ?: throw IllegalArgumentException("Invalid BlockParameter: $input")
            ).toBlockParameter()
        } else {
          (
            input.toULongOrNull(radix = 10)
              ?: throw IllegalArgumentException("Invalid BlockParameter: $input")
            ).toBlockParameter()
        }
      }
    }
  }

  enum class Tag(val value: String) : BlockParameter {
    PENDING("pending"),
    LATEST("latest"),
    EARLIEST("earliest"),
    SAFE("safe"),
    FINALIZED("finalized"),
    ;

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
  value class BlockNumber(val number: ULong) : BlockParameter {
    override fun toString(): String = number.toString()
  }

  class BlockHash(hashBytes: ByteArray) : BlockParameter {
    private val _hashBytes: ByteArray

    init {
      require(hashBytes.size == 32) { "block hash must be 32 bytes, got ${hashBytes.size}" }
      _hashBytes = hashBytes.copyOf()
    }

    val hashBytes: ByteArray get() = _hashBytes.copyOf()
    val hashHex: String get() = _hashBytes.encodeHex(prefix = true)

    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (other !is BlockHash) return false
      return _hashBytes.contentEquals(other._hashBytes)
    }

    override fun hashCode(): Int = _hashBytes.contentHashCode()

    override fun toString(): String = hashHex
  }
}

fun ULong.toBlockParameter(): BlockParameter.BlockNumber = BlockParameter.BlockNumber(this)
fun UInt.toBlockParameter(): BlockParameter.BlockNumber = BlockParameter.BlockNumber(this.toULong())
