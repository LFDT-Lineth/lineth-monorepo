package linea.coordinator.config.v2

interface FeatureToggle {
  val disabled: Boolean
}

fun FeatureToggle?.isDisabled(): Boolean = this?.disabled ?: true

fun FeatureToggle?.isEnabled(): Boolean = !isDisabled()
