package lineth.config.docs

import kotlin.reflect.KClass

/**
 * A top-level config file and its root TOML schema class.
 *
 * An application may load several distinct config files, some sharing the same top-level key, so
 * generated docs and schema snapshots are organised per file (keyed by [label]) rather than in a
 * single flat path namespace.
 *
 * @property label stable identifier for the file, used as the key in generated output.
 * @property description human-readable summary of what the file configures.
 * @property rootClass the Hoplite TOML data class the file deserialises into.
 */
data class ConfigFileRoot(
  val label: String,
  val description: String,
  val rootClass: KClass<*>,
)
