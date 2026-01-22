package net.consensys.zkevm.coordinator.api.dto

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.zkevm.coordinator.app.conflationbacktesting.ConflationBacktestingService.ConflationBacktestingRequest

data class ConflationCreateProverRequestJsonDto(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
) {
  constructor() : this(startBlockNumber = 0, endBlockNumber = 0)
  companion object {
    fun parseFrom(request: JsonRpcRequest): List<ConflationCreateProverRequestJsonDto> {
      try {
        return jacksonObjectMapper().readValue(
          jacksonObjectMapper().writeValueAsString(request.params),
          Array<ConflationCreateProverRequestJsonDto>::class.java,
        ).toList()
      } catch (e: Exception) {
        throw IllegalArgumentException("Failed to parse ConflationCreateProverRequestJsonDto from request params", e)
      }
    }

    fun toDomainObject(dto: ConflationCreateProverRequestJsonDto): ConflationBacktestingRequest {
      return ConflationBacktestingRequest(
        startBlockNumber = dto.startBlockNumber.toULong(),
        endBlockNumber = dto.endBlockNumber.toULong(),
      )
    }
  }
}
