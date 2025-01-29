# Delegation States Overview

The Babylon Staking Indexer tracks BTC delegations through various states. Each state represents a specific phase in the delegation lifecycle, triggered by different events.

## State Definitions

### 1. PENDING
- **Description**: Initial state when delegation is created
- **Triggered by**: `EventBTCDelegationCreated`
- **Purpose**: Awaiting covenant signatures

### 2. VERIFIED
- **Description**: Delegation has received required covenant signatures
- **Triggered by**: `EventCovenantQuorumReached` (pre-approval flow only)
- **Purpose**: Received covenant signatures but waiting for inclusion proof of staking tx (reported by vigilante)

### 3. ACTIVE
- **Description**: Staking inclusion proof has been received by Babylon
- **Triggered by**: 
  - Old flow: `EventCovenantQuorumReached`
  - New flow: `EventBTCDelegationInclusionProofReceived`
- **Purpose**: Delegation is active and participating in the staking protocol

### 4. UNBONDING
- **Description**: Delegation is in unbonding period
- **Triggered by**:
  - `EventBTCDelgationUnbondedEarly`: Early unbonding request
  - `EventBTCDelegationExpired`: Natural expiration
- **Purpose**: Delegation no longer contributes to voting power of staked finality provider

### 5. WITHDRAWABLE
- **Description**: Delegation can be withdrawn
- **Triggered by**: Expiry checker routine
- **Purpose**: Indicates timelock expiration (staking/unbonding/slashing), staker can withdraw now

### 6. WITHDRAWN
- **Description**: Terminal state after successful withdrawal
- **Triggered by**: Staking, Unbonding, Slashing tx output has been spent through timelock path
- **Purpose**: Terminal and final state, no more actions possible

### 7. SLASHED
- **Description**: Penalized state
- **Triggered by**: When staking or unbonding output has been spent through slashing path
- **Possible Flows**:
  - Active → Slashed → Withdrawable → Withdrawn
  - Active → Unbonding → Slashed → Withdrawable → Withdrawn
  - Active → Unbonding → Withdrawable → Slashed → Withdrawable → Withdrawn