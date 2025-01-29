package net.consensys.zkevm.ethereum

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class MakefileContractDeploymentHelperKtTest {

  @Test
  fun getDeployedAddress_messageService() {
    assertThat(
      getDeployedAddress(
        listOf(
          "L2MessageService artifact has been deployed in 1.2659626659999998s ",
          "contract=L2MessageService deployed: address=0xFE48d65B84AA0E23594Fd585c11cAD831F90AcB6 blockNumber=8",
          ""
        ),
        l2MessageServiceAddressPattern
      )
    ).isEqualTo(
      DeployedContract("0xFE48d65B84AA0E23594Fd585c11cAD831F90AcB6", 8)
    )

    assertThat(
      getDeployedAddress(
        listOf(
          "L2MessageServiceV1.2.3 artifact has been deployed in 1.2659626659999998s ",
          "contract=L2MessageServiceV1.2.3 deployed: address=0xFE48d65B84AA0E23594Fd585c11cAD831F90AcB6 blockNumber=8",
          ""
        ),
        l2MessageServiceAddressPattern
      )
    ).isEqualTo(
      DeployedContract("0xFE48d65B84AA0E23594Fd585c11cAD831F90AcB6", 8)
    )
  }

  @Test
  fun getDeployedAddress_LineaRollup() {
    assertThat(
      getDeployedAddress(
        listOf(
          "LineaRollup artifact has been deployed in 1.855172125s ",
          "contract=LineaRollup deployed: address=0x8613180dF1485B8b87DEE3BCf31896659eb1a092 blockNumber=1414",
          ""
        ),
        lineaRollupAddressPattern
      )
    ).isEqualTo(
      DeployedContract("0x8613180dF1485B8b87DEE3BCf31896659eb1a092", 1414)
    )

    assertThat(
      getDeployedAddress(
        listOf(
          "LineaRollup6.2.1 artifact has been deployed in 1.855172125s ",
          "contract=LineaRollupV6.2.1 deployed: address=0x8613180dF1485B8b87DEE3BCf31896659eb1a092 blockNumber=1414",
          ""
        ),
        lineaRollupAddressPattern
      )
    ).isEqualTo(
      DeployedContract("0x8613180dF1485B8b87DEE3BCf31896659eb1a092", 1414)
    )
  }
}
