import { describe, it, expect, afterEach } from "@jest/globals";

import { DB } from "../dataSource";

jest.mock("typeorm", () => ({
  DataSource: jest.fn().mockImplementation((config) => ({
    config,
    initialize: jest.fn(),
  })),
  Column: () => jest.fn(),
  Entity: () => jest.fn(),
  PrimaryGeneratedColumn: () => jest.fn(),
  CreateDateColumn: () => jest.fn(),
  UpdateDateColumn: () => jest.fn(),
  Index: () => jest.fn(),
  Unique: () => jest.fn(),
}));

jest.mock("typeorm-naming-strategies", () => ({
  SnakeNamingStrategy: jest.fn(),
}));

jest.mock("fs", () => ({
  readFileSync: jest.fn().mockReturnValue(Buffer.from("ca-cert-content")),
}));

describe("DB.create", () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  it("should create a DataSource with the given config", () => {
    const dbOptions = { type: "postgres" as const, host: "localhost", port: 5432 };

    const ds = DB.create(dbOptions);

    expect(ds).toBeDefined();
    expect(ds.config).toEqual(
      expect.objectContaining({
        type: "postgres",
        host: "localhost",
        port: 5432,
        migrationsTableName: "migrations",
        logging: ["error"],
        migrationsRun: true,
      }),
    );
  });

  it("should include SSL config when ssl option is provided", () => {
    const dbOptions = {
      type: "postgres" as const,
      host: "localhost",
      port: 5432,
      ssl: {
        rejectUnauthorized: true,
        ca: "/path/to/ca.pem",
      },
    };

    const ds = DB.create(dbOptions);

    expect(ds.config).toEqual(
      expect.objectContaining({
        ssl: {
          rejectUnauthorized: true,
          ca: "ca-cert-content",
        },
      }),
    );
  });

  it("should not include SSL config when ssl is not specified", () => {
    const dbOptions = { type: "postgres" as const, host: "localhost", port: 5432 };

    const ds = DB.create(dbOptions);

    expect(ds.config.ssl).toBeUndefined();
  });

  it("should handle SSL config with no CA path", () => {
    const dbOptions = {
      type: "postgres" as const,
      host: "localhost",
      port: 5432,
      ssl: {
        rejectUnauthorized: false,
      },
    };

    const ds = DB.create(dbOptions);

    expect(ds.config).toEqual(
      expect.objectContaining({
        ssl: {
          rejectUnauthorized: false,
          ca: undefined,
        },
      }),
    );
  });
});
