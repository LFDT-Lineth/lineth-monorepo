package linea.coordinator.clients.prover.serialization

import com.fasterxml.jackson.databind.DeserializationFeature
import com.fasterxml.jackson.databind.MapperFeature
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule
import com.fasterxml.jackson.module.kotlin.jacksonMapperBuilder

object JsonSerialization {
  val proofResponseMapperV1: ObjectMapper =
    jacksonMapperBuilder()
      .enable(MapperFeature.ACCEPT_CASE_INSENSITIVE_ENUMS)
      .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false)
      .addModule(JavaTimeModule())
      .build()
}
