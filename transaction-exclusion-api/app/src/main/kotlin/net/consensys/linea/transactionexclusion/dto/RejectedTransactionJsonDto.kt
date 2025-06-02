package net.consensys.linea.transactionexclusion.dto

import com.fasterxml.jackson.databind.ObjectMapper
import net.consensys.linea.transactionexclusion.ModuleOverflow
import net.consensys.linea.transactionexclusion.RejectedTransaction
import net.consensys.linea.transactionexclusion.app.api.ArgumentParser

data class ModuleOverflowJsonDto(
  val count: Long,
  val limit: Long,
  val module: String,
) {
  // Jackson ObjectMapper requires a default constructor
  constructor() : this(0L, 0L, "")

  companion object {
    fun parseListFrom(target: Any): List<ModuleOverflowJsonDto> {
      return ObjectMapper().readValue(
        ObjectMapper().writeValueAsString(target),
        Array<ModuleOverflowJsonDto>::class.java,
      ).toList()
    }
  }

  fun toDomainObject(): ModuleOverflow {
    return ModuleOverflow(
      count = count,
      limit = limit,
      module = module,
    )
  }
}

data class RejectedTransactionJsonDto(
  val txRejectionStage: String,
  val timestamp: String,
  val blockNumber: String?,
  val transactionRLP: String,
  val reasonMessage: String,
  val overflows: Any,
) {
  // Jackson ObjectMapper requires a default constructor
  constructor() : this("", "", null, "", "", Any())

  companion object {
    fun parseFrom(target: Any): RejectedTransactionJsonDto {
      return ObjectMapper().readValue(
        ObjectMapper().writeValueAsString(target),
        RejectedTransactionJsonDto::class.java,
      )
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RejectedTransactionJsonDto

    if (txRejectionStage != other.txRejectionStage) return false
    if (timestamp != other.timestamp) return false
    if (blockNumber != other.blockNumber) return false
    if (transactionRLP != other.transactionRLP) return false
    if (reasonMessage != other.reasonMessage) return false
    return overflows == other.overflows
  }

  override fun hashCode(): Int {
    var result = txRejectionStage.hashCode()
    result = 31 * result + timestamp.hashCode()
    result = 31 * result + (blockNumber?.hashCode() ?: 0)
    result = 31 * result + transactionRLP.hashCode()
    result = 31 * result + reasonMessage.hashCode()
    result = 31 * result + overflows.hashCode()
    return result
  }

  fun toDomainObject(): RejectedTransaction {
    return ArgumentParser.getTransactionRLPInRawBytes(transactionRLP)
      .let { parsedTransactionRLP ->
        RejectedTransaction(
          txRejectionStage = ArgumentParser.getTxRejectionStage(txRejectionStage),
          timestamp = ArgumentParser.getTimestampFromISO8601(timestamp),
          blockNumber = ArgumentParser.getBlockNumber(blockNumber),
          transactionRLP = parsedTransactionRLP,
          reasonMessage = ArgumentParser.getReasonMessage(reasonMessage),
          overflows = ArgumentParser.getOverflows(overflows),
          transactionInfo = ArgumentParser.getTransactionInfoFromRLP(parsedTransactionRLP),
        )
      }
  }
}
