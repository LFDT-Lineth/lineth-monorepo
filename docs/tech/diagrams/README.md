# Diagrams

This directory contains Mermaid diagram source files (`.mmd`) for the Linea architecture documentation.

## Files

### Architecture Overview

| File | Description |
|------|-------------|
| [system-architecture.mmd](./system-architecture.mmd) | Main system architecture with L1/L2 components |
| [system-dependencies.mmd](./system-dependencies.mmd) | High-level system dependency graph |
| [component-interactions.mmd](./component-interactions.mmd) | Detailed component interaction diagram |
| [component-dependency-graph.mmd](./component-dependency-graph.mmd) | Build/code dependency graph |

### Data Flow

| File | Description |
|------|-------------|
| [proving-pipeline.mmd](./proving-pipeline.mmd) | Block → Proof → Finalization flow |
| [l1-to-l2-message-flow.mmd](./l1-to-l2-message-flow.mmd) | L1 → L2 message sequence |
| [l2-to-l1-message-flow.mmd](./l2-to-l1-message-flow.mmd) | L2 → L1 message sequence |
| [message-lifecycle.mmd](./message-lifecycle.mmd) | Message state transitions |
| [data-availability-modes.mmd](./data-availability-modes.mmd) | Rollup vs Validium modes |
| [state-recovery-flow.mmd](./state-recovery-flow.mmd) | State recovery from L1 |

### ZK Proving

| File | Description |
|------|-------------|
| [zk-proving-architecture.mmd](./zk-proving-architecture.mmd) | Full ZK proving stack |
| [proof-system.mmd](./proof-system.mmd) | PLONK + Vortex proof system |
| [constraint-system.mmd](./constraint-system.mmd) | tracer-constraints compilation |
| [module-interactions.mmd](./module-interactions.mmd) | Constraint module relationships |

### Component Architecture

| File | Description |
|------|-------------|
| [sequencer-architecture.mmd](./sequencer-architecture.mmd) | Sequencer stack (Besu + Maru) |
| [coordinator-architecture.mmd](./coordinator-architecture.mmd) | Coordinator internal structure |
| [prover-architecture.mmd](./prover-architecture.mmd) | Prover internal structure |
| [controller-workflow.mmd](./controller-workflow.mmd) | Prover controller job flow |
| [tracer-architecture.mmd](./tracer-architecture.mmd) | Tracer internal structure |
| [besu-plugins-architecture.mmd](./besu-plugins-architecture.mmd) | Besu plugins overview |
| [plugin-lifecycle.mmd](./plugin-lifecycle.mmd) | Besu plugin lifecycle |
| [postman-architecture.mmd](./postman-architecture.mmd) | Postman service structure |
| [sdk-architecture.mmd](./sdk-architecture.mmd) | SDK package structure |
| [bridge-ui-architecture.mmd](./bridge-ui-architecture.mmd) | Bridge UI components |
| [contracts-architecture.mmd](./contracts-architecture.mmd) | Smart contracts structure |

### SDK Flows

| File | Description |
|------|-------------|
| [l1-to-l2-deposit-flow.mmd](./l1-to-l2-deposit-flow.mmd) | SDK deposit sequence |
| [l2-to-l1-withdrawal-flow.mmd](./l2-to-l1-withdrawal-flow.mmd) | SDK withdrawal sequence |

### Operations

| File | Description |
|------|-------------|
| [native-yield-automation.mmd](./native-yield-automation.mmd) | Native yield automation flow |
| [docker-network-topology.mmd](./docker-network-topology.mmd) | Local dev network layout |

### Testing

| File | Description |
|------|-------------|
| [e2e-test-coverage.mmd](./e2e-test-coverage.mmd) | E2E test suite structure |

## Viewing Diagrams

### GitHub

GitHub renders `.mmd` files automatically when viewed in the repository.

### VS Code

Install the "Markdown Preview Mermaid Support" extension or "Mermaid Preview" extension.

### Online

Copy the contents to [Mermaid Live Editor](https://mermaid.live/) for interactive editing.

### Command Line

Use the Mermaid CLI to generate images:

```bash
# Install
npm install -g @mermaid-js/mermaid-cli

# Generate PNG
mmdc -i system-architecture.mmd -o system-architecture.png

# Generate SVG
mmdc -i component-interactions.mmd -o component-interactions.svg

# Generate all diagrams
for f in *.mmd; do mmdc -i "$f" -o "${f%.mmd}.svg"; done
```

## Diagram Types Used

| Type | Syntax | Use Case |
|------|--------|----------|
| Flowchart | `flowchart TB/LR` | Architecture, data flow |
| Sequence | `sequenceDiagram` | Message flows, API calls |
| State | `stateDiagram-v2` | Lifecycle, transitions |

## Contributing

When adding new diagrams:

1. Use descriptive filenames (kebab-case)
2. Add a title as a Mermaid comment on the first line: `%% Diagram Title`
3. Update this README with the new diagram
4. Ensure the corresponding ASCII diagram exists in the docs

> **Note**: Avoid YAML frontmatter (`---` blocks) as it's not compatible with all Mermaid renderers including mermaid.live.
