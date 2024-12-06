# Creating your own L2 rollup testnet

This tutorial will guide you through setting up your own OP Stack testnet chain. We'll follow the official steps until the initialization of services, at which point we'll switch to using Docker Compose for simplification.

## Software dependencies

| Dependency | Version | Version Check Command |
| ---------- | ------- | --------------------- |
| git        | ^2      | git --version         |
| go         | ^1.21   | go version            |
| node       | ^20     | node --version        |
| pnpm       | ^8      | pnpm --version        |
| foundry    | ^0.2.0  | forge --version       |
| make       | ^3      | make --version        |
| jq         | ^1.6    | jq --version          |
| direnv     | ^2      | direnv --version      |

## Get access to a Sepolia node

You'll need access to a Sepolia node. You can either:
- Use a node provider like Alchemy (recommended)
- Run your own Sepolia node

## Build the source code

### Clone the repositories
```
cd ~
git clone https://github.com/ethereum-optimism/optimism.git
git clone https://github.com/ethereum-optimism/op-geth.git
```

### Build op-geth
```
cd op-geth
make geth
```

## Environment Setup

Create environment files:
```
# Copy example environment file
cp .envrc.example .envrc

# Allow direnv to load the environment
direnv allow
```

The .envrc file contains important private keys and configurations. Make sure to:
- Never commit this file to git
- Keep it secure and backed up
- Use different keys for production deployments

Fund these addresses on Sepolia:
- Admin — 0.5 Sepolia ETH
- Proposer — 0.2 Sepolia ETH
- Batcher — 0.1 Sepolia ETH

## Deploy L1 Contracts

Go to `optimism` monorepo:
```
git clone https://github.com/ethereum-optimism/optimism.git
cd ~/optimism

# generate wallets if needed and fill .envrc file with private keys
# cp .envrc.example .envrc

direnv allow
```

Install dependencies and generate configuration:
```
cd ~/optimism/packages/contracts-bedrock
pnpm install
./scripts/getting-started/config.sh
```

Update the configuration file:
```
vim deploy-config/getting-started.json
```

Deploy L1 contracts:
```
forge script scripts/Deploy.s.sol:Deploy --private-key $GS_ADMIN_PRIVATE_KEY --broadcast --rpc-url $L1_RPC_URL --slow
```

This will create a .deploy directory containing the addresses of deployed contracts.

Generate l2-allocs file:
```
CONTRACT_ADDRESSES_PATH=./deployments/getting-started/.deploy \
DEPLOY_CONFIG_PATH=./deploy-config/getting-started.json \
STATE_DUMP_PATH=./l2-allocs.json \
  forge script scripts/L2Genesis.s.sol:L2Genesis \
  --sig 'runWithStateDump()'
```

Bring all the files from `optimism` monorepo to the current directory:
```
cp ~/optimism/packages/contracts-bedrock/deploy-config/getting-started.json .
cp -r ~/optimism/packages/contracts-bedrock/deployments/getting-started/.deploy .
cp ~/optimism/packages/contracts-bedrock/l2-allocs.json .
```




## Generate L2 Config Files

Generate genesis files:
```
cd ~/optimism/op-node
go run cmd/main.go genesis l2 \
  --deploy-config ../packages/contracts-bedrock/deploy-config/getting-started.json \
  --l1-deployments ../packages/contracts-bedrock/deployments/getting-started/.deploy \
  --outfile.l2 genesis.json \
  --outfile.rollup rollup.json \
  --l1-rpc $L1_RPC_URL
```

Generate JWT secret:
```
openssl rand -hex 32 > jwt.txt
```

# 🔄 Switching to Docker Compose

At this point, instead of running each component individually as in the original tutorial, we'll use Docker Compose to orchestrate all services.

Copy necessary files to working directory:
```
cp ~/optimism/op-node/genesis.json .
cp ~/optimism/op-node/rollup.json .
cp -r ~/optimism/packages/contracts-bedrock/deployments/getting-started/.deploy .
```

Start the services:
```
docker compose up -d
```

This starts:
- op-geth: L2 execution engine
- op-node: Rollup node that processes L1 data
- op-batcher: Submits L2 transactions to L1
- op-proposer: Submits L2 output proposals to L1

## Verify Your Setup

Check if services are running:
```
docker compose ps
```

Check op-node synchronization:
```
curl -d '{"id":0,"jsonrpc":"2.0","method":"optimism_syncStatus","params":[]}' \
  -H "Content-Type: application/json" http://localhost:9545
```

Check op-geth synchronization:
```
curl -d '{"id":0,"jsonrpc":"2.0","method":"eth_syncing","params":[]}' \
  -H "Content-Type: application/json" http://localhost:8545
```

## Connect to Your Chain

Your L2 chain is available at:
- HTTP RPC: http://localhost:8545
- WebSocket RPC: ws://localhost:8546

Add it to MetaMask:
- Network Name: Optimism L2 Testnet
- RPC URL: http://localhost:8545
- Chain ID: 8041
- Currency Symbol: ETH

## Important Files

Keep these files secure:
- .deploy/ - Deployed contract addresses
- genesis.json - L2 genesis configuration
- rollup.json - Rollup configuration
- jwt.txt - Authentication secret
- .envrc - Environment configuration

## Troubleshooting

If services fail to start, check logs:
```
docker compose logs -f
```

To reset the chain:
```
docker compose down
rm -rf op-geth/datadir
docker compose up -d
```

## Next Steps

- Deploy smart contracts to your L2
- Configure additional L2 nodes
- Set up monitoring and alerts
- Explore the [OP Stack Documentation](https://docs.optimism.io/stack/getting-started)

## References

- [Official Tutorial](https://docs.optimism.io/builders/chain-operators/tutorials/create-l2-rollup)
- [OP Stack Documentation](https://docs.optimism.io/stack/getting-started)
- [Optimism GitHub](https://github.com/ethereum-optimism/optimism)