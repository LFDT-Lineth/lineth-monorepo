package linea.web3j

import linea.domain.AccessListEntry
import linea.domain.Transaction
import linea.domain.TransactionType
import linea.domain.toBesu
import linea.kotlin.decodeHex
import linea.kotlin.toBigInteger
import linea.kotlin.toBigIntegerFromHex
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.fail
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Wei
import org.junit.jupiter.api.Test
import org.web3j.protocol.ObjectMapperFactory
import org.web3j.protocol.core.methods.response.EthBlock
import kotlin.jvm.optionals.getOrElse
import kotlin.jvm.optionals.getOrNull

class EthGetBlockToLineaBlockMapperTest {
  // using raw JSON from eth_getBlockByNumber responses because realistically represens our use case
  // Also it's very easy create new test cases
  private fun serialize(json: String): EthBlock.TransactionObject {
    return ObjectMapperFactory.getObjectMapper().readValue(json, EthBlock.TransactionObject::class.java)
  }

  @Test
  fun `should map frontier transactions without chainId replay protection and null yParity field`() {
    val txWeb3j = serialize(
      """
      {
          "blockHash": "0x8de5957e6b5b519eb889a49604e96d7ace847475a9c3ccfaf0acc87e89175d0f",
          "blockNumber": "0x1",
          "from": "0x1b9abeec3215d8ade8a33607f2cf0f4f60e5f0d0",
          "gas": "0x29e2f7",
          "gasPrice": "0x7",
          "maxFeePerGas": "0xe",
          "maxPriorityFeePerGas": "0x0",
          "hash": "0x09ffe43152572dedf9d4c893b0721692fa20a63d74deb8ff6b9d1ce74c1fd17d",
          "input": "0x60806040523480156200001157600080fd5b506200001c",
          "nonce": "0x0",
          "to": null,
          "transactionIndex": "0x0",
          "value": "0x0",
          "type": "0x2",
          "accessList": [],
          "chainId": "0x539",
          "v": "0x0",
          "r": "0x1fa31b9272cc67174efb129c2fd2ec5afda122503745beb22bd26e48a42240bb",
          "s": "0x248c9cdf9352b4a379577c5b44bcb25a5350dc6722fd7b2aec40e193f670e4f4"
      }
      """.trimIndent()
    )

    val domainTx = txWeb3j.toDomain()
    assertThat(domainTx).isEqualTo(
      Transaction(
        nonce = 0x0UL,
        gasPrice = null,
        gasLimit = 0x29e2f7UL,
        to = null,
        value = 0UL.toBigInteger(),
        input = "0x60806040523480156200001157600080fd5b506200001c".decodeHex(),
        r = "0x1fa31b9272cc67174efb129c2fd2ec5afda122503745beb22bd26e48a42240bb".toBigIntegerFromHex(),
        s = "0x248c9cdf9352b4a379577c5b44bcb25a5350dc6722fd7b2aec40e193f670e4f4".toBigIntegerFromHex(),
        v = 0UL,
        yParity = null,
        type = TransactionType.EIP1559,
        chainId = 0x539UL,
        maxFeePerGas = 0xeUL,
        maxPriorityFeePerGas = 0x0UL,
        accessList = emptyList()
      )
    )

    domainTx.toBesu().also { besuTx ->
      assertThat(besuTx.type).isEqualTo(org.hyperledger.besu.datatypes.TransactionType.EIP1559)
      assertThat(besuTx.nonce).isEqualTo(0x0L)
      assertThat(besuTx.gasPrice.getOrNull()).isNull()
      assertThat(besuTx.maxFeePerGas.getOrNull()).isEqualTo(Wei.of(0xeL))
      assertThat(besuTx.maxPriorityFeePerGas.getOrNull()).isEqualTo(Wei.of(0x0L))
      assertThat(besuTx.gasLimit).isEqualTo(0x29e2f7L)
      assertThat(besuTx.to.getOrNull()).isNull()
      assertThat(besuTx.value).isEqualTo(Wei.of(0x0L))
      assertThat(besuTx.payload).isEqualTo(Bytes.fromHexString("0x60806040523480156200001157600080fd5b506200001c"))
      assertThat(besuTx.signature.r).isEqualTo(
        "0x1fa31b9272cc67174efb129c2fd2ec5afda122503745beb22bd26e48a42240bb".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.s).isEqualTo(
        "0x248c9cdf9352b4a379577c5b44bcb25a5350dc6722fd7b2aec40e193f670e4f4".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.recId).isEqualTo(0)
      assertThat(besuTx.chainId.getOrNull()).isEqualTo(0x539L)
    }
  }

  @Test
  fun `should map frontier transactions`() {
    val txWeb3j = serialize(
      """
      {
        "blockHash": "0x004257e560a5f82595dddb73f752b904efef4b73cb3ece1469f5e5091e3c9665",
        "blockNumber": "0xe1d30",
        "chainId": "0xe705",
        "from": "0x228466f2c715cbec05deabfac040ce3619d7cf0b",
        "gas": "0x5208",
        "gasPrice": "0xee2d984",
        "hash": "0x5d3b5e1ae3e4ea5612e6907cb09c4e0e5482171b4c2af794e17b77314547bb79",
        "input": "0x",
        "nonce": "0x97411",
        "r": "0xdf28597129341d5d345c9043c7d0b0a22be82cac13988cfc1d8cbdaf3ab3f35b",
        "s": "0x3189b2ff80d8f728d6fb7503b46734ee77a60a42db01d0b09db10bdc9d5caa44",
        "to": "0x228466f2c715cbec05deabfac040ce3619d7cf0b",
        "transactionIndex": "0x0",
        "type": "0x0",
        "v": "0x1ce2e",
        "value": "0x186a0"
      }
      """.trimIndent()
    )
    val domainTx = txWeb3j.toDomain()
    assertThat(domainTx).isEqualTo(
      Transaction(
        nonce = 0x97411UL,
        gasPrice = 0xee2d984UL,
        gasLimit = 0x5208UL,
        to = "0x228466f2c715cbec05deabfac040ce3619d7cf0b".decodeHex(),
        value = 0x186a0UL.toBigInteger(),
        input = "0x".decodeHex(),
        r = "0xdf28597129341d5d345c9043c7d0b0a22be82cac13988cfc1d8cbdaf3ab3f35b".toBigIntegerFromHex(),
        s = "0x3189b2ff80d8f728d6fb7503b46734ee77a60a42db01d0b09db10bdc9d5caa44".toBigIntegerFromHex(),
        v = 118318UL,
        yParity = null,
        type = TransactionType.FRONTIER,
        chainId = 0xe705UL,
        maxFeePerGas = null,
        maxPriorityFeePerGas = null,
        accessList = null
      )
    )
    domainTx.toBesu().also { besuTx ->
      assertThat(besuTx.type).isEqualTo(org.hyperledger.besu.datatypes.TransactionType.FRONTIER)
      assertThat(besuTx.nonce).isEqualTo(0x97411L)
      assertThat(besuTx.gasPrice.getOrNull()).isEqualTo(Wei.of(0xee2d984L))
      assertThat(besuTx.gasLimit).isEqualTo(0x5208L)
      assertThat(besuTx.to.getOrNull()).isEqualTo(Address.fromHexString("0x228466f2c715cbec05deabfac040ce3619d7cf0b"))
      assertThat(besuTx.value).isEqualTo(Wei.of(0x186a0L))
      assertThat(besuTx.payload).isEqualTo(Bytes.EMPTY)
      assertThat(besuTx.signature.r).isEqualTo(
        "0xdf28597129341d5d345c9043c7d0b0a22be82cac13988cfc1d8cbdaf3ab3f35b".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.s).isEqualTo(
        "0x3189b2ff80d8f728d6fb7503b46734ee77a60a42db01d0b09db10bdc9d5caa44".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.recId).isEqualTo(1)
      assertThat(besuTx.chainId.getOrNull()).isEqualTo(0xe705L)
      assertThat(besuTx.maxFeePerGas).isEmpty()
      assertThat(besuTx.maxPriorityFeePerGas).isEmpty()
    }
  }

  @Test
  fun `should map transaction with AccessList`() {
    val txWeb3j = serialize(
      """
      {
        "accessList": [
          {
            "address": "0x8d97689c9818892b700e27f316cc3e41e17fbeb9",
            "storageKeys": [
              "0x0000000000000000000000000000000000000000000000000000000000000000",
              "0x0000000000000000000000000000000000000000000000000000000000000001"
            ]
          }
        ],
        "blockHash": "0x7480ae911853c1fba10145401a21ddca3943b5894d74cbbf7a6beec526d1f9c2",
        "blockNumber": "0xa",
        "chainId": "0x539",
        "from": "0xce3b7d471fd1fdd10d788ae64e48a9c2f2361179",
        "gas": "0x30d40",
        "gasPrice": "0x1017df87",
        "hash": "0x8ef620582ed8ba98c8496a42b27a30ff7b1de901b1ff7e65b22ea59a2d0668ce",
        "input": "0x",
        "nonce": "0x0",
        "to": "0x8d97689c9818892b700e27f316cc3e41e17fbeb9",
        "transactionIndex": "0x2",
        "type": "0x1",
        "value": "0x2386f26fc10000",
        "yParity": "0x1",
        "v": "0x1",
        "r": "0x4f24ed24207bec8591c8172584dc3b57cdf3ee96afbd5e63905a90a704ff33f0",
        "s": "0x6277bb9d2614843a4791ff2c192e70876438ec940c39d92deb504591b83dfeb3"
      }
      """.trimIndent()
    )

    val domainTx = txWeb3j.toDomain()
    assertThat(domainTx).isEqualTo(
      Transaction(
        nonce = 0UL,
        gasPrice = 0x1017df87UL,
        gasLimit = 0x30d40UL,
        to = "0x8d97689c9818892b700e27f316cc3e41e17fbeb9".decodeHex(),
        value = 0x2386f26fc10000UL.toBigInteger(),
        input = "0x".decodeHex(),
        r = "0x4f24ed24207bec8591c8172584dc3b57cdf3ee96afbd5e63905a90a704ff33f0".toBigIntegerFromHex(),
        s = "0x6277bb9d2614843a4791ff2c192e70876438ec940c39d92deb504591b83dfeb3".toBigIntegerFromHex(),
        v = 1UL,
        yParity = 1UL,
        type = TransactionType.ACCESS_LIST,
        chainId = 0x539UL,
        maxFeePerGas = null,
        maxPriorityFeePerGas = null,
        accessList = listOf(
          AccessListEntry(
            address = "0x8d97689c9818892b700e27f316cc3e41e17fbeb9".decodeHex(),
            listOf(
              "0x0000000000000000000000000000000000000000000000000000000000000000".decodeHex(),
              "0x0000000000000000000000000000000000000000000000000000000000000001".decodeHex()
            )
          )
        )
      )
    )

    domainTx.toBesu().also { besuTx ->
      assertThat(besuTx.nonce).isEqualTo(0L)
      assertThat(besuTx.gasPrice.getOrNull()).isEqualTo(Wei.of(0x1017df87L))
      assertThat(besuTx.gasLimit).isEqualTo(0x30d40L)
      assertThat(besuTx.to.getOrNull()).isEqualTo(Address.fromHexString("0x8d97689c9818892b700e27f316cc3e41e17fbeb9"))
      assertThat(besuTx.value).isEqualTo(Wei.of(0x2386f26fc10000L))
      assertThat(besuTx.payload).isEqualTo(Bytes.EMPTY)
      assertThat(besuTx.signature.r).isEqualTo(
        "0x4f24ed24207bec8591c8172584dc3b57cdf3ee96afbd5e63905a90a704ff33f0".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.s).isEqualTo(
        "0x6277bb9d2614843a4791ff2c192e70876438ec940c39d92deb504591b83dfeb3".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.recId).isEqualTo(1)
      assertThat(besuTx.type).isEqualTo(org.hyperledger.besu.datatypes.TransactionType.ACCESS_LIST)
      assertThat(besuTx.chainId.getOrNull()).isEqualTo(0x539L)
      assertThat(besuTx.maxFeePerGas).isEmpty()
      assertThat(besuTx.maxPriorityFeePerGas).isEmpty()
      val accessList = besuTx.accessList.getOrElse { fail("AccessList is empty") }

      assertThat(accessList.get(0).address)
        .isEqualTo(Address.fromHexString("0x8d97689c9818892b700e27f316cc3e41e17fbeb9"))
      assertThat(accessList.get(0).storageKeys)
        .containsExactly(
          Bytes32.fromHexString("0x0000000000000000000000000000000000000000000000000000000000000000"),
          Bytes32.fromHexString("0x0000000000000000000000000000000000000000000000000000000000000001")
        )
    }
  }

  @Test
  fun `should map type accessList with empty list`() {
    val txWeb3j = serialize(
      """
        {
          "accessList": [],
          "blockHash": "0x7480ae911853c1fba10145401a21ddca3943b5894d74cbbf7a6beec526d1f9c2",
          "blockNumber": "0xa",
          "chainId": "0x539",
          "from": "0x5007b0259849a673d0d780611f9a2ed8821d9ebe",
          "gas": "0x5208",
          "gasPrice": "0x1017df87",
          "hash": "0xa2334c8858bb44ef3e9ef7f3523ec058ab24a869cfad7333fdf7bf3bb76deec4",
          "input": "0x",
          "nonce": "0x0",
          "to": "0x8d97689c9818892b700e27f316cc3e41e17fbeb9",
          "transactionIndex": "0x3",
          "type": "0x1",
          "value": "0x2386f26fc10000",
          "yParity": "0x1",
          "v": "0x1",
          "r": "0xc57273f9ba15320937d5d9dfd1dc0b18d1e678b34bd3a4bfd29a63e11a856292",
          "s": "0x7aa875a64835ecc5f9ac1a9fe3ab38d2a62bb3643a2597ab585a5607641a0c57"
        }
      """.trimIndent()
    )

    val domainTx = txWeb3j.toDomain()
    assertThat(domainTx).isEqualTo(
      Transaction(
        nonce = 0UL,
        gasPrice = 0x1017df87UL,
        gasLimit = 0x5208UL,
        to = "0x8d97689c9818892b700e27f316cc3e41e17fbeb9".decodeHex(),
        value = 0x2386f26fc10000UL.toBigInteger(),
        input = "0x".decodeHex(),
        r = "0xc57273f9ba15320937d5d9dfd1dc0b18d1e678b34bd3a4bfd29a63e11a856292".toBigIntegerFromHex(),
        s = "0x7aa875a64835ecc5f9ac1a9fe3ab38d2a62bb3643a2597ab585a5607641a0c57".toBigIntegerFromHex(),
        v = 1UL,
        yParity = 1UL,
        type = TransactionType.ACCESS_LIST,
        chainId = 0x539UL,
        maxFeePerGas = null,
        maxPriorityFeePerGas = null,
        accessList = emptyList()
      )
    )

    domainTx.toBesu().also { besuTx ->
      assertThat(besuTx.type).isEqualTo(org.hyperledger.besu.datatypes.TransactionType.ACCESS_LIST)
      assertThat(besuTx.nonce).isEqualTo(0L)
      assertThat(besuTx.gasPrice.getOrNull()).isEqualTo(Wei.of(0x1017df87L))
      assertThat(besuTx.gasLimit).isEqualTo(0x5208L)
      assertThat(besuTx.to.getOrNull()).isEqualTo(Address.fromHexString("0x8d97689c9818892b700e27f316cc3e41e17fbeb9"))
      assertThat(besuTx.value).isEqualTo(Wei.of(0x2386f26fc10000L))
      assertThat(besuTx.payload).isEqualTo(Bytes.EMPTY)
      assertThat(besuTx.signature.r).isEqualTo(
        "0xc57273f9ba15320937d5d9dfd1dc0b18d1e678b34bd3a4bfd29a63e11a856292".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.s).isEqualTo(
        "0x7aa875a64835ecc5f9ac1a9fe3ab38d2a62bb3643a2597ab585a5607641a0c57".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.recId).isEqualTo(1)
      assertThat(besuTx.chainId.getOrNull()).isEqualTo(0x539L)
      assertThat(besuTx.maxFeePerGas).isEmpty()
      assertThat(besuTx.maxPriorityFeePerGas).isEmpty()
      // it shall have an empty accessList
      assertThat(besuTx.accessList.getOrNull()).isEmpty()
    }
  }

  @Test
  fun `it should map EIP1559 tx`() {
    val input =
      """
        0xdeb3cdf2000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000003e84d538bf9753309729adf92867d75bc2e58b566cb204b0d1b018fbf311e4b49bc52c1c450b2f412f5d9a44e01990d6127bc805ba8ef079ed2a897070378d706fbd2f5cf52b0e172541b7f11b9c2f0e0b91a67c5caf5908ddfdd2d340349e7398b698b3c336876c88e232f0e8f3197f2683e54a4439abb7d210d84cc1d3ad1bd48d0ba5dc30714d253743a734f17e88354eea550f7945df35d4c6316fcfaad09846f81f59b8127b037dbce6e5ffa45120fc69e6852f0c8ac2fcc9fd5e72503dd1c8d114ee079ffed84ddd6851e438cdd2a0ed1df9f0255481dc2a61a4808d856525619c948fbbd063bfd3db42504547db68c29990540eb7a36a1a8a0e483544eb634ae33f43f5bac2d991b9f6b36e23a7a299ade5b30ab96ad6dae27a9c374ae5f702fc689f596450c467722f24b7621ba5663ed6e08b620f04bc524338cac50e9ebc302d0b33dd9e2e563f05ce26303666a6c8c0f8dcd0b475f2219398ff4552533d28660c8d0d2843ce238ffb856dee06bc28e1ec0b92c3cb7b91378c07b049f3af20017dbdfdf48320ea7cf5f331bee27ba33d6a41351b3f044612a45f51451c068c23d6aec6784f623c6855acb95f07f213ad8605861fd8601ffa9a0282508a4d859769cb61247389020587a570cba1eac8b05576bd5b7a81b166f3c7b0f0ae0a8117f642d3fd0957e1cface4d10ebe6475a9a1f3bf6e3b1b7c16e50e529adcf0cf278aa64b9fecedcb0d894ab7ba6589e96ff56dff7b8636413f46cdc073c22521f6b89d7b68ca6f8af1ecc4e453137c801a9b35b5c4869883f59aaeaa7ee637d71c7e02f08894cffdb51a368b225fa1ab00e3ad2d91d1275d048aad5eb5d34438622c7aea1759b3fe747c2b0fddd62159de1d7cabbccde9c1e3511a34432e0c4e6dede019e38493fc29292ea321621629c1ffc62160747ee136171c96f55af7af6a29b8ca94f12d12b7c706974b1e586b3674a6aec7510f1025ba399f7a97f5911187b040b7a494e191bd761ad2a78daf427f5ee19cf24bc45fab34d32747de0f0a2c6bd33d2440d9f5d20da22da34e418d54d6894d42edd6d0c5a4f9b02d510a23db40cc455b7c423bd43b6fcd0f3655285e16ba8d9bdaf3f2147de572c33568353b5f5dd820f49dbddbc63297aed5e2f342b383a83319f9beed9d3d358a3dc7c0a010b85954fec3b34c3227a9b4447bd5d30b8b78c0ef36cd8197e867d37778b24e8ebfaadf08c42f3db6e5e46cb025bb4e98334ce0a7a59ba155eaf3968621f353d075e0d68f0787259e344a72e8938ebe3a81458ad20df917dc1392fe759210f045f7d87177ad39a13ec704301f1f0845b8c6cbc52f8f77c043bcc80adb513470d0e5a6b02df65259bcc3198efe01c555a5d28bf89f818ea1b984a64db220f487e230652000000000000000000000000000000000000000000000000
      """.trimIndent().trim()
    val txWeb3j = serialize(
      """
        {
          "accessList": [],
          "blockHash": "0x7480ae911853c1fba10145401a21ddca3943b5894d74cbbf7a6beec526d1f9c2",
          "blockNumber": "0xa",
          "chainId": "0x539",
          "from": "0x5007b0259849a673d0d780611f9a2ed8821d9ebe",
          "gas": "0x5208",
          "gasPrice": "0x1017df87",
          "hash": "0xa2334c8858bb44ef3e9ef7f3523ec058ab24a869cfad7333fdf7bf3bb76deec4",
          "input": "$input",
          "nonce": "0x0",
          "to": "0xe4392c8ecc46b304c83cdb5edaf742899b1bda93",
          "transactionIndex": "0x3",
          "type": "0x2",
          "value": "0x2386f26fc10000",
          "yParity": "0x1",
          "v": "0x1",
          "r": "0xeb4f70991ea4f14d23efb32591da3621d551406fd32bdfdd78bb677dec13160a",
          "s": "0x783aaa89f73ef7535924da8fd5f12e15cae1a0811c4c4746d1c23abff1eacddf",
          "maxFeePerGas": "0x1017dff7",
          "maxPriorityFeePerGas": "0x1017df87"
        }
      """.trimIndent()
    )

    val txDomain = txWeb3j.toDomain()
    assertThat(txDomain).isEqualTo(
      Transaction(
        nonce = 0UL,
        // when type is EIP1559 gasPrice is null,
        // eth_getBlock returns effectiveGasPrice but we will place as null here
        gasPrice = null,
        gasLimit = 0x5208UL,
        to = "0xe4392c8ecc46b304c83cdb5edaf742899b1bda93".decodeHex(),
        value = 0x2386f26fc10000UL.toBigInteger(),
        input = input.decodeHex(),
        r = "0xeb4f70991ea4f14d23efb32591da3621d551406fd32bdfdd78bb677dec13160a".toBigIntegerFromHex(),
        s = "0x783aaa89f73ef7535924da8fd5f12e15cae1a0811c4c4746d1c23abff1eacddf".toBigIntegerFromHex(),
        v = 1UL,
        yParity = 1UL,
        type = TransactionType.EIP1559,
        chainId = 0x539UL,
        maxFeePerGas = 0x1017dff7UL,
        maxPriorityFeePerGas = 0x1017df87UL,
        accessList = emptyList()
      )
    )

    txDomain.toBesu().also { txBesu ->
      assertThat(txBesu.type).isEqualTo(org.hyperledger.besu.datatypes.TransactionType.EIP1559)
      assertThat(txBesu.nonce).isEqualTo(0L)
      assertThat(txBesu.gasPrice.getOrNull()).isNull()
      assertThat(txBesu.gasLimit).isEqualTo(0x5208L)
      assertThat(txBesu.to.getOrNull()).isEqualTo(Address.fromHexString("0xe4392c8ecc46b304c83cdb5edaf742899b1bda93"))
      assertThat(txBesu.value).isEqualTo(Wei.of(0x2386f26fc10000L))
      assertThat(txBesu.payload).isEqualTo(Bytes.fromHexString(input))
      assertThat(txBesu.signature.r).isEqualTo(
        "0xeb4f70991ea4f14d23efb32591da3621d551406fd32bdfdd78bb677dec13160a".toBigIntegerFromHex()
      )
      assertThat(txBesu.signature.s).isEqualTo(
        "0x783aaa89f73ef7535924da8fd5f12e15cae1a0811c4c4746d1c23abff1eacddf".toBigIntegerFromHex()
      )
      assertThat(txBesu.signature.recId).isEqualTo(1)
      assertThat(txBesu.chainId.getOrNull()).isEqualTo(0x539L)
      assertThat(txBesu.maxFeePerGas.getOrNull()).isEqualTo(Wei.of(0x1017dff7L))
      assertThat(txBesu.maxPriorityFeePerGas.getOrNull()).isEqualTo(Wei.of(0x1017df87L))
      assertThat(txBesu.accessList.getOrNull()).isEmpty()
    }
  }

  @Test
  fun `shall decode tx with to=null`() {
    val input = """
      0x608060405234801561001057600080fd5b5061001a3361001f565b61006f565b600080546001600160a01b038381166001600160a01b0319831681178455604051919092169283917f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e09190a35050565b6108658061007e6000396000f3fe60806040526004361061007b5760003560e01c80639623609d1161004e5780639623609d1461012b57806399a88ec41461013e578063f2fde38b1461015e578063f3b7dead1461017e57600080fd5b8063204e1c7a14610080578063715018a6146100c95780637eff275e146100e05780638da5cb5b14610100575b600080fd5b34801561008c57600080fd5b506100a061009b366004610608565b61019e565b60405173ffffffffffffffffffffffffffffffffffffffff909116815260200160405180910390f35b3480156100d557600080fd5b506100de610255565b005b3480156100ec57600080fd5b506100de6100fb36600461062c565b610269565b34801561010c57600080fd5b5060005473ffffffffffffffffffffffffffffffffffffffff166100a0565b6100de610139366004610694565b6102f7565b34801561014a57600080fd5b506100de61015936600461062c565b61038c565b34801561016a57600080fd5b506100de610179366004610608565b6103e8565b34801561018a57600080fd5b506100a0610199366004610608565b6104a4565b60008060008373ffffffffffffffffffffffffffffffffffffffff166040516101ea907f5c60da1b00000000000000000000000000000000000000000000000000000000815260040190565b600060405180830381855afa9150503d8060008114610225576040519150601f19603f3d011682016040523d82523d6000602084013e61022a565b606091505b50915091508161023957600080fd5b8080602001905181019061024d9190610788565b949350505050565b61025d6104f0565b6102676000610571565b565b6102716104f0565b6040517f8f28397000000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff8281166004830152831690638f283970906024015b600060405180830381600087803b1580156102db57600080fd5b505af11580156102ef573d6000803e3d6000fd5b505050505050565b6102ff6104f0565b6040517f4f1ef28600000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff841690634f1ef28690349061035590869086906004016107a5565b6000604051808303818588803b15801561036e57600080fd5b505af1158015610382573d6000803e3d6000fd5b5050505050505050565b6103946104f0565b6040517f3659cfe600000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff8281166004830152831690633659cfe6906024016102c1565b6103f06104f0565b73ffffffffffffffffffffffffffffffffffffffff8116610498576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602660248201527f4f776e61626c653a206e6577206f776e657220697320746865207a65726f206160448201527f646472657373000000000000000000000000000000000000000000000000000060648201526084015b60405180910390fd5b6104a181610571565b50565b60008060008373ffffffffffffffffffffffffffffffffffffffff166040516101ea907ff851a44000000000000000000000000000000000000000000000000000000000815260040190565b60005473ffffffffffffffffffffffffffffffffffffffff163314610267576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820181905260248201527f4f776e61626c653a2063616c6c6572206973206e6f7420746865206f776e6572604482015260640161048f565b6000805473ffffffffffffffffffffffffffffffffffffffff8381167fffffffffffffffffffffffff0000000000000000000000000000000000000000831681178455604051919092169283917f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e09190a35050565b73ffffffffffffffffffffffffffffffffffffffff811681146104a157600080fd5b60006020828403121561061a57600080fd5b8135610625816105e6565b9392505050565b6000806040838503121561063f57600080fd5b823561064a816105e6565b9150602083013561065a816105e6565b809150509250929050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b6000806000606084860312156106a957600080fd5b83356106b4816105e6565b925060208401356106c4816105e6565b9150604084013567ffffffffffffffff808211156106e157600080fd5b818601915086601f8301126106f557600080fd5b81358181111561070757610707610665565b604051601f82017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0908116603f0116810190838211818310171561074d5761074d610665565b8160405282815289602084870101111561076657600080fd5b8260208601602083013760006020848301015280955050505050509250925092565b60006020828403121561079a57600080fd5b8151610625816105e6565b73ffffffffffffffffffffffffffffffffffffffff8316815260006020604081840152835180604085015260005b818110156107ef578581018301518582016060015282016107d3565b5060006060828601015260607fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0601f83011685010192505050939250505056fea2646970667358221220688ab5dd8d9528556ea321a4b4ef35edd0288c19274db8bf4057c8b61d9e438764736f6c63430008130033
    """.trimIndent().trim()
    val txWeb3j = serialize(
      """
        {
          "accessList": [],
          "blockHash": "0xf9bf74ade4a723a5527badeb62ce58d478f1022df0effc2a091898ef068563b6",
          "blockNumber": "0x1",
          "chainId": "0x539",
          "from": "0x1b9abeec3215d8ade8a33607f2cf0f4f60e5f0d0",
          "gas": "0x83a3d",
          "gasPrice": "0x7",
          "maxPriorityFeePerGas": "0x0",
          "maxFeePerGas": "0xe",
          "hash": "0xc9647251765f5d679e024dd0e5c0f4700c431f129e50847c3f73e2aa2262e593",
          "input": "$input",
          "nonce": "0x1",
          "to": null,
          "transactionIndex": "0x1",
          "type": "0x2",
          "value": "0x0",
          "yParity": "0x1",
          "v": "0x1",
          "r": "0xf7afccb560d0c52bea021ba522a27dbd6c3aba3512dd2d3b2f476ed8dd87d5f7",
          "s": "0x5f47f6ddcf1c216eb33eb69db553d682de34c78f5a5ab97905a428c2182f32e"
        }
      """.trimIndent()
    )

    val txDomain = txWeb3j.toDomain()
    assertThat(txDomain).isEqualTo(
      Transaction(
        nonce = 1UL,
        gasLimit = 0x83a3dUL,
        to = null,
        value = 0UL.toBigInteger(),
        input = input.decodeHex(),
        r = "0xf7afccb560d0c52bea021ba522a27dbd6c3aba3512dd2d3b2f476ed8dd87d5f7".toBigIntegerFromHex(),
        s = "0x5f47f6ddcf1c216eb33eb69db553d682de34c78f5a5ab97905a428c2182f32e".toBigIntegerFromHex(),
        v = 1UL,
        yParity = 1UL,
        type = TransactionType.EIP1559,
        chainId = 0x539UL,
        gasPrice = null,
        maxFeePerGas = 0xeUL,
        maxPriorityFeePerGas = 0UL,
        accessList = emptyList()
      )
    )

    txDomain.toBesu().let { txBesu ->
      assertThat(txBesu.type).isEqualTo(org.hyperledger.besu.datatypes.TransactionType.EIP1559)
      assertThat(txBesu.nonce).isEqualTo(1L)
      assertThat(txBesu.gasPrice.getOrNull()).isNull()
      assertThat(txBesu.gasLimit).isEqualTo(0x83a3dL)
      assertThat(txBesu.to.getOrNull()).isNull()
      assertThat(txBesu.value).isEqualTo(Wei.ZERO)
      assertThat(txBesu.payload).isEqualTo(Bytes.fromHexString(input))
      assertThat(txBesu.signature.r).isEqualTo(
        "0xf7afccb560d0c52bea021ba522a27dbd6c3aba3512dd2d3b2f476ed8dd87d5f7".toBigIntegerFromHex()
      )
      assertThat(txBesu.signature.s).isEqualTo(
        "0x5f47f6ddcf1c216eb33eb69db553d682de34c78f5a5ab97905a428c2182f32e".toBigIntegerFromHex()
      )
      assertThat(txBesu.signature.recId).isEqualTo(1)
      assertThat(txBesu.chainId.getOrNull()).isEqualTo(0x539L)
      assertThat(txBesu.maxFeePerGas.getOrNull()).isEqualTo(Wei.of(0xeL))
      assertThat(txBesu.maxPriorityFeePerGas.getOrNull()).isEqualTo(Wei.ZERO)
      assertThat(txBesu.accessList.getOrNull()).isEmpty()
    }
  }

  @Test
  fun `should map unprotected or no_chainId transactions`() {
    val input = """
      0x1688f0b9000000000000000000000000fb1bffc9d739b8d520daf37df666da4c687191ea000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000189ef999caf00000000000000000000000000000000000000000000000000000000000001e4b63e800d00000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001c0000000000000000000000000017062a1de2fe6b99be3d9d37841fed19f57380400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000050000000000000000000000006dad852ec59e420b4d0f10ec700c2a3193982b8900000000000000000000000080b09aad7351f430d747135631851114d9a29386000000000000000000000000ebb8788f11d5cb1cb42156363ad619eb4bd37e3e000000000000000000000000d200d041015dba3a71931e87a2a02c1a8f9fe374000000000000000000000000732e38862cc96df5573fb075756831f8940d531b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
    """.trimIndent()
    val txWeb3j = serialize(
      """{
          "blockHash": "0x0733f57cf576ce9b0ca47ea9959c1c7ec7f39d26460439a92893034755fc5a93",
          "blockNumber": "0x6e6af2",
          "from": "0xfa2930a9d96e91b0a7e78d9febb7a8c744afd0da",
          "gas": "0x1e8480",
          "gasPrice": "0x11e1a300",
          "hash": "0x7a1bfa2a0c7dd2af549190448330dadc85c459e2851030a6b81fb2ce99286239",
          "input": "$input",
          "nonce": "0x1",
          "r": "0x98cf46978ebd95f2f61780c767b1ad392beaa11b68f0e310728f5be8296e752a",
          "s": "0x1c621c3046755e5600d73b83a0c28676b02a7dff6b89f76b02f5eddd7817854",
          "to": "0x4e1dcf7ad4e460cfd30791ccc4f9c8a4f820ec67",
          "transactionIndex": "0x3",
          "type": "0x0",
          "v": "0x1b",
          "value": "0x0"
      }
      """.trimIndent()
    )
    val domainTx = txWeb3j.toDomain()
    assertThat(domainTx).isEqualTo(
      Transaction(
        nonce = 0x1UL,
        gasPrice = 0x11e1a300UL,
        gasLimit = 0x1e8480UL,
//        gasLimit = 0x2e8480UL,
        to = "0x4e1dcf7ad4e460cfd30791ccc4f9c8a4f820ec67".decodeHex(),
        value = 0x0.toBigInteger(),
        input = input.decodeHex(),
        r = "0x98cf46978ebd95f2f61780c767b1ad392beaa11b68f0e310728f5be8296e752a".toBigIntegerFromHex(),
        s = "0x1c621c3046755e5600d73b83a0c28676b02a7dff6b89f76b02f5eddd7817854".toBigIntegerFromHex(),
        v = 0x1bUL,
        yParity = null,
        type = TransactionType.FRONTIER,
        chainId = null,
        maxFeePerGas = null,
        maxPriorityFeePerGas = null,
        accessList = null
      )
    )
    domainTx.toBesu().also { besuTx ->
      assertThat(besuTx.type).isEqualTo(org.hyperledger.besu.datatypes.TransactionType.FRONTIER)
      assertThat(besuTx.nonce).isEqualTo(0x1L)
      assertThat(besuTx.gasPrice.getOrNull()).isEqualTo(Wei.of(0x11e1a300UL.toLong()))
      assertThat(besuTx.gasLimit).isEqualTo(0x1e8480L)
      assertThat(besuTx.to.getOrNull()).isEqualTo(Address.fromHexString("0x4e1dcf7ad4e460cfd30791ccc4f9c8a4f820ec67"))
      assertThat(besuTx.value).isEqualTo(Wei.of(0x0))
      assertThat(besuTx.payload).isEqualTo(Bytes.fromHexString(input))
      assertThat(besuTx.signature.r).isEqualTo(
        "0x98cf46978ebd95f2f61780c767b1ad392beaa11b68f0e310728f5be8296e752a".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.s).isEqualTo(
        "0x1c621c3046755e5600d73b83a0c28676b02a7dff6b89f76b02f5eddd7817854".toBigIntegerFromHex()
      )
      assertThat(besuTx.signature.recId).isEqualTo(0)
      assertThat(besuTx.chainId.getOrNull()).isNull()
      assertThat(besuTx.maxFeePerGas).isEmpty()
      assertThat(besuTx.maxPriorityFeePerGas).isEmpty()
    }
  }
}
