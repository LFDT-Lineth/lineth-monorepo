import { PostgresConnectionOptions } from "typeorm/driver/postgres/PostgresConnectionOptions";

export type DBOptions = PostgresConnectionOptions;

export type DBCleanerOptions = {
  enabled: boolean;
  cleaningInterval?: number;
  daysBeforeNowToDelete?: number;
};

export type DBCleanerConfig = Required<DBCleanerOptions>;
