package net.consensys.zkevm.load;

import net.consensys.zkevm.load.model.*;
import org.apache.commons.cli.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class ManualLoadTest {
  private static final Logger logger = LoggerFactory.getLogger(ManualLoadTest.class);

  /**
   * Call main method by passing arguments like '-ft 2 -nbTx 2'
   *
   * @param args
   * @throws Exception
   */
  public static void main(String[] args) throws Exception {
    // create Options object
    var options = getOptions();
    var parser = new DefaultParser();

    try {
      // parse the command line arguments
      CommandLine cmd = parser.parse(options, args);

      if (cmd.hasOption("help")) {
        String header = "Tools to generate load on an ethereum network.";
        HelpFormatter formatter = new HelpFormatter();
        formatter.printHelp("LoadTesting", header, options, "", true);
        return;
      }

      if (cmd.hasOption("request")) {
        TestExecutor testExecutor =
            new TestExecutor(cmd.getOptionValue("request"), cmd.getOptionValue("pk"));
        testExecutor.test();
      }
    } catch (ParseException e) {
      logger.info("Failed to parse command line properties." + e);
    }
  }

  private static Options getOptions() {
    Options options = new Options();
    options.addOption("request", true, "request file describing the test");

    // add option "private key of source wallet"
    options.addOption("pk", "sourceWalletPK", true, "private key of source wallet");

    return options;
  }
}
