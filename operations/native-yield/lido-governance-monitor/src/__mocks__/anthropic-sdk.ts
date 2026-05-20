import { jest } from "@jest/globals";

class MockAnthropic {
  messages = {
    create: jest.fn(),
  };

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  constructor(_options?: { apiKey?: string }) {}
}

export default MockAnthropic;
