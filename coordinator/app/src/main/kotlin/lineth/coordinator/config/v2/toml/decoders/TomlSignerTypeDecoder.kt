package lineth.coordinator.config.v2.toml.decoders

import linea.hoplite.toml.TomlEnumDecoder
import lineth.coordinator.config.v2.toml.SignerConfigToml

class TomlSignerTypeDecoder : TomlEnumDecoder<SignerConfigToml.SignerType>(SignerConfigToml.SignerType::class.java)
