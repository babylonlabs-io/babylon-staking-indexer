# Startup Flow

The Babylon Staking Indexer follows a specific startup sequence to ensure proper synchronization with the babylon chain and database. Here's the detailed startup flow:

## 1. Initial Connections
- Establishes connection to Babylon node
- Connects to Bitcoin node
- Initializes indexer database connection
- Starts metrics server

## 2. Main Service Routines
The service starts four major concurrent routines:

### 2.1 Parameter Synchronization
- Syncs Babylon BTCStaking module parameters
- Syncs Babylon checkpointing module parameters

### 2.2 BTC Notification Resubscription
- Resubscribes to delegations that might have lost BTC notifier subscriptions
- Handles service restart recovery
- States monitored: Active, Unbonding, Withdrawable, Slashed

### 2.3 Expiry Checker
- Monitors delegation expiry times
- Checks staking timelock expiry
- Checks unbonding timelock expiry
- Checks slashing timelock expiry
- Marks eligible delegations as withdrawable

### 2.4 Babylon Block Subscription
- Establishes WebSocket connection for new blocks
- Maintains real-time block updates

### 2.5 Block Processing
- Bootstraps from genesis to latest block
- Processes each block sequentially
- Extracts and parses relevant events
- Updates delegation and finality provider states