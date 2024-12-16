package net.consensys.linea

sealed interface BlockParameter {
  fun getTag(): String

  fun getNumber(): ULong

  companion object {
    fun fromNumber(blockNumber: Number): BlockNumber {
      require(blockNumber.toLong() >= 0) { "block number must be greater or equal than 0, value=$blockNumber" }
      return BlockNumber(blockNumber.toLong().toULong())
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
  }
}
