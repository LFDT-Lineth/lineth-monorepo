package net.consensys.linea.web3j

/*
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.methods.response.AccessListObject
import org.web3j.protocol.core.methods.response.EthBlock

class DomainObjectMappersTest {

  private val signedContractCreation = "0xf901b7808459682f07834421868080b90165608060405234801561001057600080fd5b50610" +
    "145806100206000396000f3fe608060405234801561001057600080fd5b506004361061002b5760003560e01c8063f15da729146100305" +
    "75b600080fd5b6100a76004803603602081101561004657600080fd5b81019080803590602001906401000000008111156100635760008" +
    "0fd5b82018360208201111561007557600080fd5b8035906020019184600183028401116401000000008311171561009757600080fd5b9" +
    "0919293919293905050506100a9565b005b7fdb84d7c006c4de68f9c0bd50b8b81ed31f29ebeec325c872d36445c6565d757c828260405" +
    "180806020018281038252848482818152602001925080828437600081840152601f19601f8201169050808301925050509350505050604" +
    "05180910390a1505056fea265627a7a72315820943da63deb39c746f6663430903701509d1219fda43468ef09d46a87bf7ffb2564736f6" +
    "c634300051000321ca05d891d090909165b7020a03780ed5954684d1db53c23387eaab3dd1d14163bcea02dbbf71810bc49dd79bc2ab03" +
    "779d4436ba4c8232c27f04a96eb923e6e31ed8f"

  private val signedEIP1159 = "0x02f86b05825d39801782520894f36f155486299ecaff2d4f5160ed5114c1f660008732123526d198bc" +
    "80c080a039df7e43c747f44fec95ff470c9b0b1be26dfc49b20558bf7180feab68d7fdcea06d9fd7fc7f48be55735b7d64102cde9effff" +
    "eb956de37307560908656778d93f"

  private val signedEIP2930EmptyAccessList = "0x01f8f682e70481a98506f0fb17038302d26c9403555948c82a6f473b28b1e7541dc" +
    "91d1927d52d872c68af0bb14000b8849c87a12100000000000000000000000000000000000000000000000000000000000000400000000" +
    "00000000000000000000000000000000000000000000000000966018000000000000000000000000000000000000000000000000000000" +
    "000000000086c6f646566693331000000000000000000000000000000000000000000000000c001a0e6681acfeadef3ed6f2817169ae45" +
    "d3741d74a1d11cea2405258890ddf556c1ba04a70cb870f96a69048f6085f4203c0ce4ceffae6fbdec00a7fb41f370be1e213"

  private val signedFrontier = "0xf88754830186a08346612494b6479d58eacc73705237571cd34c6c40c809865480a4fdacd57600000" +
    "000000000000000000000000000000000000000000000000000000000011ca09876ddc44c63163cfe454312df7d67149b4ec3fe54debe3" +
    "963620d01754d6e8da065c18e6b0c6e120c76b9114121a4805e67a3515c5bce17da3769e432687609f8"

  private val signedEIP2930 = "0x01f9015282e70481a98506f0fb17038302d26c9403555948c82a6f473b28b1e7541dc91d1927d52d87" +
    "2c68af0bb14000b8849c87a121000000000000000000000000000000000000000000000000000000000000004000000000000000000000" +
    "0000000000000000000000000000000000000966018000000000000000000000000000000000000000000000000000000000000000086c" +
    "6f646566693331000000000000000000000000000000000000000000000000f85bf85994009e7baea6a6c7c4c2dfeb977efac326af552d" +
    "87f842a00000000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000" +
    "000000000000000000000000000101a0e6681acfeadef3ed6f2817169ae45d3741d74a1d11cea2405258890ddf556c1ba04a70cb870f96" +
    "a69048f6085f4203c0ce4ceffae6fbdec00a7fb41f370be1e213"

  @Test
  fun encodeSignedEIP1559() {
    val encodedTransaction = EthBlock.TransactionObject(
      /*hash*/"",
      /*nonce*/"0x5D39",
      /*blockHash*/"",
      /*blockNumber*/"",
      /*chainId*/"5",
      /*transactionIndex*/"",
      /*from*/"",
      /*to*/"0xf36F155486299eCAff2D4F5160ed5114C1f66000",
      /*value*/"0x32123526D198BC",
      /*gasPrice*/"0x",
      /*gas*/"0x5208",
      /*input*/"0x",
      /*creates*/"",
      /*publicKey*/"",
      /*raw*/"",
      /*r*/"0x39df7e43c747f44fec95ff470c9b0b1be26dfc49b20558bf7180feab68d7fdce",
      /*s*/"0x6d9fd7fc7f48be55735b7d64102cde9effffeb956de37307560908656778d93f",
      /*v*/0,
      /*yParity*/"",
      /*type*/"0x2",
      /*maxFeePerGas*/"0x17",
      /*maxPriorityFeePerGas*/"0x0",
      /*accessList*/null
    ).toBytes()

    assertThat(encodedTransaction.toString()).isEqualTo(signedEIP1159)
  }

  @Test
  fun encodeSignedFrontier() {
    val encodedTransaction = EthBlock.TransactionObject(
      /*hash*/"",
      /*nonce*/"0x54",
      /*blockHash*/"",
      /*blockNumber*/"",
      /*chainId*/null,
      /*transactionIndex*/"",
      /*from*/"",
      /*to*/"0xb6479d58Eacc73705237571CD34C6C40C8098654",
      /*value*/"0x0",
      /*gasPrice*/"0x186A0",
      /*gas*/"0x466124",
      /*input*/"0xfdacd5760000000000000000000000000000000000000000000000000000000000000001",
      /*creates*/"",
      /*publicKey*/"",
      /*raw*/"",
      /*r*/"0x9876ddc44c63163cfe454312df7d67149b4ec3fe54debe3963620d01754d6e8d",
      /*s*/"0x65c18e6b0c6e120c76b9114121a4805e67a3515c5bce17da3769e432687609f8",
      /*v*/28,
      /*yParity*/"",
      /*type*/"0x0",
      /*maxFeePerGas*/null,
      /*maxPriorityFeePerGas*/null,
      /*accessList*/null
    ).toBytes()

    assertThat(encodedTransaction.toString()).isEqualTo(signedFrontier)
  }

  @Test
  fun encodeSignedAccessList() {
    val accessList = listOf(
      AccessListObject(
        "0x9e7baea6a6c7c4c2dfeb977efac326af552d87",
        listOf(
          "0x0000000000000000000000000000000000000000000000000000000000000000",
          "0x0000000000000000000000000000000000000000000000000000000000000001"
        )
      )
    )

    val encodedTransaction = EthBlock.TransactionObject(
      /*hash*/ "",
      /*nonce*/ "0xA9",
      /*blockHash*/ "",
      /*blockNumber*/ "",
      /*chainId*/ "0xe704",
      /*transactionIndex*/ "",
      /*from*/ "",
      /*to*/ "0x03555948c82a6f473b28b1e7541dc91d1927d52d",
      /*value*/ "0x2C68AF0BB14000",
      /*gasPrice*/ "0x6f0fb1703",
      /*gas*/ "0x2d26c",
      /*input*/
      "0x9c87a12100000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000" +
        "00000000000000000000000000000966018000000000000000000000000000000000000000000000000000000000000000086c6f64" +
        "6566693331000000000000000000000000000000000000000000000000",
      /*creates*/"",
      /*publicKey*/"",
      /*raw*/"",
      /*r*/"0xe6681acfeadef3ed6f2817169ae45d3741d74a1d11cea2405258890ddf556c1b",
      /*s*/"0x4a70cb870f96a69048f6085f4203c0ce4ceffae6fbdec00a7fb41f370be1e213",
      /*v*/0x1,
      /*yParity*/ "",
      /*type*/"0x1",
      /*maxFeePerGas*/null,
      /*maxPriorityFeePerGas*/null,
      /*accessList*/accessList
    ).toBytes()

    assertThat(encodedTransaction.toString()).isEqualTo(signedEIP2930)
  }

  @Test
  fun encodeSignedAccessListEmptyList() {
    val emptyAccessList = emptyList<AccessListObject>()

    val encodedTransaction = EthBlock.TransactionObject(
      /*hash=*/"",
      /*nonce*/"0xA9",
      /*blockHash*/ "",
      /*blockNumber*/"",
      /*chainId*/"0xe704",
      /*transactionIndex*/"",
      /*from*/"",
      /*to*/"0x03555948c82a6f473b28b1e7541dc91d1927d52d",
      /*value*/"0x2C68AF0BB14000",
      /*gasPrice*/"0x6f0fb1703",
      /*gas*/"0x2d26c",
      /*input*/
      "0x9c87a12100000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000" +
        "00000000000000000000000000000966018000000000000000000000000000000000000000000000000000000000000000086c6f64" +
        "6566693331000000000000000000000000000000000000000000000000",
      /*creates*/"",
      /*publicKey*/"",
      /*raw*/"",
      /*r*/"0xe6681acfeadef3ed6f2817169ae45d3741d74a1d11cea2405258890ddf556c1b",
      /*s*/"0x4a70cb870f96a69048f6085f4203c0ce4ceffae6fbdec00a7fb41f370be1e213",
      /*v*/0x1,
      /*yParity*/ "",
      /*type*/"0x1",
      /*maxFeePerGas*/null,
      /*maxPriorityFeePerGas*/null,
      /*accessList*/ emptyAccessList
    ).toBytes()

    assertThat(encodedTransaction.toString()).isEqualTo(signedEIP2930EmptyAccessList)
  }

  @Test
  fun encodeContractCreation() {
    val encodedTransaction = EthBlock.TransactionObject(
      /*hash*/"",
      /*nonce*/"0x0",
      /*blockHash*/"",
      /*blockNumber*/"",
      /*chainId*/null,
      /*transactionIndex*/"",
      /*from*/"",
      /*to*/null,
      /*value*/"0x0",
      /*gasPrice*/"0x59682f07",
      /*gas*/"0x442186",
      /*input*/
      "0x608060405234801561001057600080fd5b50610145806100206000396000f3fe608060405234801561001057600080fd5b50" +
        "6004361061002b5760003560e01c8063f15da72914610030575b600080fd5b6100a76004803603602081101561004657600080fd5b" +
        "810190808035906020019064010000000081111561006357600080fd5b82018360208201111561007557600080fd5b803590602001" +
        "9184600183028401116401000000008311171561009757600080fd5b90919293919293905050506100a9565b005b7fdb84d7c006c4" +
        "de68f9c0bd50b8b81ed31f29ebeec325c872d36445c6565d757c828260405180806020018281038252848482818152602001925080" +
        "828437600081840152601f19601f820116905080830192505050935050505060405180910390a1505056fea265627a7a7231582094" +
        "3da63deb39c746f6663430903701509d1219fda43468ef09d46a87bf7ffb2564736f6c63430005100032",
      /*creates*/ "",
      /*publicKey*/ "",
      /*raw*/ "",
      /*r*/ "0x5d891d090909165b7020a03780ed5954684d1db53c23387eaab3dd1d14163bce",
      /*s*/ "0x2dbbf71810bc49dd79bc2ab03779d4436ba4c8232c27f04a96eb923e6e31ed8f",
      /*v*/ 28,
      /*yParity*/"",
      /*type*/ "0x0",
      /*maxFeePerGas*/ null,
      /*maxPriorityFeePerGas*/ null,
      /*accessList*/null
    ).toBytes()

    assertThat(encodedTransaction.toString()).isEqualTo(signedContractCreation)
  }
}
*/
