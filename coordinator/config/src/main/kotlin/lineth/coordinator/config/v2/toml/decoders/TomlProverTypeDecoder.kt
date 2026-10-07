package lineth.coordinator.config.v2.toml.decoders

import linea.hoplite.toml.TomlEnumDecoder
import lineth.coordinator.config.v2.toml.ProverToml

class TomlProverTypeDecoder : TomlEnumDecoder<ProverToml.ProverType>(ProverToml.ProverType::class.java)
