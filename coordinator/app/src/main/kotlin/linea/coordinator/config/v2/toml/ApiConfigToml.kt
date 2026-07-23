package linea.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.coordinator.config.v2.ApiConfig

data class ApiConfigToml(
  @param:ConfigDoc(description = "Port serving observability endpoints (metrics/health).", default = "9545")
  val observabilityPort: UInt = 9545u,
  @param:ConfigDoc(
    description = "Port serving the coordinator JSON-RPC API. 0 picks a random free port.",
    default = "0",
  )
  val jsonRpcPort: UInt = 0u,
  @param:ConfigDoc(description = "HTTP path the JSON-RPC API is served on.", default = "/")
  val jsonRpcPath: String = "/",
  @param:ConfigDoc(
    description = "Number of Vert.x verticles serving the JSON-RPC API.",
    default = "1",
  )
  val jsonRpcServerVerticles: Int = 1,
) {
  fun reified(): ApiConfig {
    return ApiConfig(
      observabilityPort = observabilityPort,
      jsonRpcPort = jsonRpcPort,
      jsonRpcPath = jsonRpcPath,
      jsonRpcServerVerticles = jsonRpcServerVerticles,
    )
  }
}
