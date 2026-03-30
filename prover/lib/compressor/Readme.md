## Building the blob compressor

### Build the library

* The Go part
```
go build -ldflags "-s -w" -buildmode=c-shared -o libcompressor.so libcompressor.go
```

* The Java (Kotlin) part

See [jvm-libs/linea/blob-compressor/src/main/kotlin/linea/blob/GoNativeBlobCompressor.kt](../../../jvm-libs/linea/blob-compressor/src/main/kotlin/linea/blob/GoNativeBlobCompressor.kt).

* [How to update the dictionary](dictionary-update-guide.md)
