package linea.coordinator.config.v2.toml.decoders

import linea.coordinator.config.v2.toml.SignerConfigToml
import linea.hoplite.toml.TomlEnumDecoder

class TomlSignerTypeDecoder : TomlEnumDecoder<SignerConfigToml.SignerType>(SignerConfigToml.SignerType::class.java)
