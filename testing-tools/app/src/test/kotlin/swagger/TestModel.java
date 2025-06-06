package swagger;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import net.consensys.zkevm.load.model.JSON;
import net.consensys.zkevm.load.model.swagger.*;
import net.consensys.zkevm.load.swagger.*;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

public class TestModel {

  public static void main(String args[]) throws JsonProcessingException {
    var request = new Request();
    request.setId(1234);
    request.setName("test");

    var ScenarioDefinition = new ScenarioDefinition();
    var tx = new MoneyTransfer();
    tx.setNbTransfers(10);
    tx.setNbWallets(10);
    ScenarioDefinition.setTransaction(tx);
    request.addCallsItem(ScenarioDefinition);

    var ScenarioDefinition2 = new ScenarioDefinition();
    var tx2 = new ContractCall();
    var contract = new CallExistingContract();
    contract.contractAddress("address");
    tx2.contract(contract);
    Mint mint = new Mint();

    contract.methodAndParameters(mint);
    ScenarioDefinition2.setTransaction(tx2);
    request.addCallsItem(ScenarioDefinition2);

    ObjectMapper om = new ObjectMapper();
    String json = om.writeValueAsString(request);
    System.out.println(json);

    var builder = JSON.createGson();
    var source = builder.create().fromJson(json, Request.class);
    Assertions.assertEquals(json, om.writeValueAsString(source));
  }

  @Test
  public void jsonTest() throws JsonProcessingException {

    Assertions.assertTrue(true);
  }
}
