import { BetterSqlite3ConnectionOptions } from "typeorm/driver/better-sqlite3/BetterSqlite3ConnectionOptions";
import { PostgresConnectionOptions } from "typeorm/driver/postgres/PostgresConnectionOptions";

export type DBOptions = PostgresConnectionOptions | BetterSqlite3ConnectionOptions;

export type DBCleanerOptions = {
  enabled: boolean;
  cleaningInterval?: number;
  daysBeforeNowToDelete?: number;
};

export type DBCleanerConfig = Required<DBCleanerOptions>;
