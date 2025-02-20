package linea.domain

sealed interface BlockParameter {
  fun getTag(): String

  fun getNumber(): ULong

  companion object {
    fun fromNumber(blockNumber: Number): BlockNumber {
      require(blockNumber.toLong() >= 0) { "block number must be greater or equal than 0, value=$blockNumber" }
      return BlockNumber(blockNumber.toLong().toULong())
    }

    fun parse(input: String): BlockParameter {
      return try {
        // Try to parse the input as a tag
        Tag.fromString(input)
      } catch (e: IllegalArgumentException) {
        // If it's not a valid tag, try to parse it as a block number
        val blockNumber = if (input.startsWith("0x")) {
          input.drop(2).toULongOrNull(radix = 16)
        } else {
          input.toULongOrNull(radix = 10)
        } ?: throw IllegalArgumentException("Invalid BlockParameter: $input")

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
    FINALIZED("finalized");

    override fun getTag(): String = value
    override fun getNumber(): ULong = throw UnsupportedOperationException(
      "getNumber isn't supposed to be called on a block tag!"
    )

    companion object {
      @JvmStatic
      fun fromString(value: String): Tag = kotlin.runCatching { Tag.valueOf(value.uppercase()) }
        .getOrElse {
          throw IllegalArgumentException(
            "BlockParameter Tag=$value is invalid. Valid values: ${Tag.entries.joinToString(", ")}"
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
}
