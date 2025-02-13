FROM ethpandaops/ethereum-genesis-generator:pk910-bash-el-genesis-generator

COPY ./generate-genesis.sh /config/generate-genesis.sh

ENTRYPOINT [ "/bin/bash", "/config/generate-genesis.sh" ]
