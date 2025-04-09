package net.consensys.linea.web3j

import linea.domain.Constants
import linea.web3j.domain.Eip4844Transaction
import org.apache.tuweni.bytes.Bytes
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.web3j.crypto.Blob
import org.web3j.protocol.ObjectMapperFactory
import org.web3j.utils.Numeric
import java.math.BigInteger

class Eip4844TransactionTest {
  private val objectMapper = ObjectMapperFactory.getObjectMapper()

  @Test
  fun canBeSerializedCorrectly() {
    val blobByteArray = ByteArray(Constants.Eip4844BlobSize)
    val blobVersionHashes = "0x010657f37554c781402a22917dee2f75def7ab966d7b770905398eba3c444014"
    val eip4844Transaction = Eip4844Transaction(
      from = "0x123",
      nonce = BigInteger.valueOf(1),
      gasPrice = BigInteger.valueOf(2),
      gasLimit = BigInteger.valueOf(4),
      to = "0x123",
      value = BigInteger.valueOf(5),
      data = "0xdata",
      chainId = 6L,
      maxPriorityFeePerGas = BigInteger.valueOf(7),
      maxFeePerGas = BigInteger.valueOf(8),
      _maxFeePerBlobGas = BigInteger.valueOf(9),
      blobs = listOf(Blob(blobByteArray)),
      blobVersionedHashes = listOf(Bytes.fromHexString(blobVersionHashes))
    )

    val serializedBlob = Numeric.toHexString(blobByteArray)
    val result = objectMapper.writeValueAsString(eip4844Transaction)
    val expectedJsonString = """{
        "blobs" : [ "$serializedBlob" ],
        "from" : "0x123",
        "to" : "0x123",
        "gas" : "0x4",
        "gasPrice" : "0x2",
        "value" : "0x5",
        "data" : "0xdata",
        "nonce" : "0x1",
        "chainId" : "0x6",
        "maxPriorityFeePerGas" : "0x7",
        "maxFeePerGas" : "0x8",
        "maxFeePerBlobGas" : "0x9",
        "blobVersionedHashes" : [ "0x010657f37554c781402a22917dee2f75def7ab966d7b770905398eba3c444014" ]
      }"""
    val expectedJsonTree = objectMapper.readTree(expectedJsonString)
    val actualTree = objectMapper.readTree(result)
    assertThat(actualTree).isEqualTo(expectedJsonTree)
  }
}
