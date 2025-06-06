package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.ApiConfig

data class ApiConfigToml(
  val observabilityPort: UInt = 9545u,
) {
  fun reified(): ApiConfig {
    return ApiConfig(observabilityPort)
  }
}
