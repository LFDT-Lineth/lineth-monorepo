# Blob Compression Dictionary Update Guide

The process is simple. The only subtle point is that support for new dictionaries should be added downstream-first (i.e. first in the prover and the decompressor, and then in the blob compressor,) and that conversely retiring a dictionary should be done upstream-first (i.e. first in the blob compressor, and then in the prover, and never in a decompressor used for state reconstruction.)

## Updating the Prover
Download the dictionary file on the machine running the prover. Add its path relative to the prover root, to the prover configuration `.toml` file, in the `dict_paths` property, under `blob_decompression`. Normally the dictionary would already be available in the Linea repository, its path looking like `"lib/compressor/dict/yy-mm-dd.bin"`.
The next time the prover is started, it will (also) load the new dictionary. Note that there is no need to recompile the decompression circuit.

Example:
```toml
[blob_decompression]
dict_paths = [
  "/path/to/old/dictionary",
  "/path/to/new/dictionary"
]
```

## Updating the Decompressor
The JNI library has a function `LoadDictionaries` available that can be used to add the new dictionary, similarly to the prover. There is not even a need to restart the process.

## Updating the Blob Compressor
Unlike the prover and decompressor, the blob compressor cannot support multiple dictionaries. The `Init` function, takes as its second input the relative path of the dictionary file. The dictionary is loaded when the compressor is initialized, and it will be used for all subsequent compression operations. This initialization function can be called multiple times, and not only at the beginning of the process. But it will reset the compressor's cache, so any blocks written into an embryonic blob will be erased.
