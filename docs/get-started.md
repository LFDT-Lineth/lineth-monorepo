# Get Started

### Requirements:

- Node.js v20 or higher
- Docker v24 or higher
  - Docker should ideally have ~16GB of Memory and 4+ CPUs to run the entire stack.
- Docker Compose version v2.19+
- Make v3.81+
- Pnpm >=v9.14.4 (https://pnpm.io/installation)

### Run stack locally

#### Install Node dependencies

```
make pnpm-install
```

#### Start stack & run E2E tests

```
make fresh-start-all

cd e2e
pnpm run test:e2e:local
```

To stop that stack run:

```
make clean-enviroment
```

While running the end2end tests, you should observe files being generated in `tmp/local/` directory.

```
├── local
│  ├── prover
│  │  ├── request
│  │  │  └── 4-4-etv0.2.0-stv1.2.0-getZkProof.json
│  │  ├── requests-done
│  │  │  ├── 1-1-etv0.2.0-stv1.2.0-getZkProof.json.success
│  │  │  ├── 2-2-etv0.2.0-stv1.2.0-getZkProof.json.success
│  │  │  └── 3-3-etv0.2.0-stv1.2.0-getZkProof.json.success
│  │  └── response
│  │      ├── 1-1-etv0.2.0-stv1.2.0-getZkProof.json.0.2.0.json
│  │      ├── 2-2-etv0.2.0-stv1.2.0-getZkProof.json.0.2.0.json
│  │      └── 3-3-etv0.2.0-stv1.2.0-getZkProof.json.0.2.0.json
│  └── traces
│      ├── conflated
│      │  ├── 1-1.conflated.v0.2.0.json.gz
│      │  ├── 2-2.conflated.v0.2.0.json.gz
│      │  ├── 3-3.conflated.v0.2.0.json.gz
│      │  └── 4-4.conflated.v0.2.0.json.gz
│      ├── raw
│      │  ├── 1-0x2e1a3f506c0d5f11310301a86f608d840d3db0e28c545eaf9e9c9812e2b795e0.v0.2.0.json.gz
│      │  ├── 2-0x3e5b3bd8e21a94488bf93776480271d3fef8033152effd4e19fe6519dea53379.v0.2.0.json.gz
│      │  ├── 3-0xa5046c13502a619a7a3f091b397234dc020f6cbda1942d247d1003d4c73899b6.v0.2.0.json.gz
│      │  ├── 4-0xe9203ede2114bf9c291692c4bd2dcc7207973c267ed411d65568d1138b3ecfcc.v0.2.0.json.gz
│      │  ├── 5-0x2c8ec07d4222bed8285be3de83f0fccc989134c49826baed5340bf7aa8e3ce8f.v0.2.0.json.gz
│      │  └── 6-0x3c7b7ee369d5fe02a6865415a2d0ef4ec385812351723e35a3b54d972f9f4ceb.v0.2.0.json.gz
│      └── raw-non-canonical
```

#### Troubleshooting

- Docker: Sometimes restarting the stack several times may lead to network/state issues. The following commands may help. **Note:** Please be aware that this will permanently remove all docker images, containers and **docker volumes** and any data saved it them.

```
make clean-environment
docker system prune --volumes
```

## Tuning in conflation

For local testing and development conflation deadline is set to 6s `conflation-deadline=PT6S` in `config/coordinator/coordinator-docker.config.toml` file. Hence, only 2 blocks conflation. If you want bigger conflations, increase the deadline accordingly.

## Next steps

Consider reviewing the [Linea architecture](architecture-description.md) description.
