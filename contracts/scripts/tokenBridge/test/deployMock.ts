import { deployTokenBridgeWithMockMessaging } from "./deployTokenBridges.js";

deployTokenBridgeWithMockMessaging(true)
  .then(() => {
    process.exitCode = 0;
    process.exit();
  })
  .catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
