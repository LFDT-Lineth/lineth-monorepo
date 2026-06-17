#!/bin/sh
set -eu

bin/blockscout eval "Elixir.Explorer.ReleaseTasks.create_and_migrate()"
exec bin/blockscout start
