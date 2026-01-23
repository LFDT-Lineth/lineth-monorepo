package linea.coordinator.config.v2

data class ApiConfig(
  val observabilityPort: UInt,
  val jsonRpcPort: UInt,
  val jsonRpcPath: String,
  val jsonRpcServerVerticles: Int,
)
