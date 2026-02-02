package net.consensys.zkevm.coordinator.api.dto

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import linea.blob.BlobCompressorVersion
import linea.kotlin.toURL
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.zkevm.coordinator.app.conflationbacktesting.ConflationBacktestingConfig
import net.consensys.zkevm.coordinator.app.conflationbacktesting.ShomeiApiConfig
import net.consensys.zkevm.coordinator.app.conflationbacktesting.TracesApiConfig

data class ConflationCreateProverRequestJsonDto(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
  val blobCompressorVersion: String,
  val batchesFixedSize: Int?,
  val parentBlobShnarf: String?,
  val tracesApi: TracesApiDto,
  val shomeiApi: ShomeiApiDto,
) {
  constructor() : this(
    startBlockNumber = 0,
    endBlockNumber = 0,
    blobCompressorVersion = "",
    batchesFixedSize = null,
    parentBlobShnarf = null,
    tracesApi = TracesApiDto(),
    shomeiApi = ShomeiApiDto(),
  )

  fun toDomainObject(): ConflationBacktestingConfig {
    return ConflationBacktestingConfig(
      startBlockNumber = this.startBlockNumber.toULong(),
      endBlockNumber = this.endBlockNumber.toULong(),
      blobCompressorVersion = BlobCompressorVersion.valueOf(this.blobCompressorVersion),
      batchesFixedSize = this.batchesFixedSize?.toUInt(),
      parentBlobShnarf = this.parentBlobShnarf,
      tracesApi = this.tracesApi.toDomainObject(),
      shomeiApi = this.shomeiApi.toDomainObject(),
    )
  }

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
  }
}

data class TracesApiDto(
  val endpoint: String,
  val version: String,
  val requestLimitPerEndpoint: Int,
) {
  constructor() : this(
    endpoint = "",
    version = "",
    requestLimitPerEndpoint = 0,
  )

  fun toDomainObject(): TracesApiConfig {
    return TracesApiConfig(
      endpoint = this.endpoint.toURL(),
      version = this.version,
      requestLimitPerEndpoint = this.requestLimitPerEndpoint.toUInt(),
    )
  }
}

data class ShomeiApiDto(
  val endpoint: String,
  val version: String,
  val requestLimitPerEndpoint: Int,
) {
  constructor() : this(
    endpoint = "",
    version = "",
    requestLimitPerEndpoint = 0,
  )
  fun toDomainObject(): ShomeiApiConfig {
    return ShomeiApiConfig(
      endpoint = this.endpoint.toURL(),
      version = this.version,
      requestLimitPerEndpoint = this.requestLimitPerEndpoint.toUInt(),
    )
  }
}
