package lineth.coordinator.config.v2.toml.decoders

import lineth.coordinator.config.v2.toml.SignerConfigToml
import lineth.hoplite.toml.TomlEnumDecoder

class TomlSignerTypeDecoder : TomlEnumDecoder<SignerConfigToml.SignerType>(SignerConfigToml.SignerType::class.java)
