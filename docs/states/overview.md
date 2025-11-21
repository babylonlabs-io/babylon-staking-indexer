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
  - `EventBTCDelgationUnbondedEarly`: Early unbonding request (sets sub-state to `EARLY_UNBONDING`)
  - `EventBTCDelegationExpired`: Natural expiration (sets sub-state to `TIMELOCK`)
- **Purpose**: Delegation no longer contributes to voting power of staked finality provider
- **Sub-States**: `TIMELOCK` or `EARLY_UNBONDING` (see Sub-State Definitions below)

### 5. WITHDRAWABLE
- **Description**: Delegation can be withdrawn
- **Triggered by**: Expiry checker routine
- **Purpose**: Indicates timelock expiration (staking/unbonding/slashing), staker can withdraw now
- **Sub-States**: `TIMELOCK`, `EARLY_UNBONDING`, `TIMELOCK_SLASHING`, or `EARLY_UNBONDING_SLASHING` (see Sub-State Definitions below)

### 6. WITHDRAWN
- **Description**: Terminal state after successful withdrawal
- **Triggered by**: Staking, Unbonding, Slashing tx output has been spent through timelock path
- **Purpose**: Terminal and final state, no more actions possible
- **Sub-States**: `TIMELOCK`, `EARLY_UNBONDING`, `TIMELOCK_SLASHING`, or `EARLY_UNBONDING_SLASHING` (see Sub-State Definitions below)

### 7. EXPANDED
- **Description**: Terminal state after delegation is expanded/extended to a new one with an extended timelock
- **Triggered by**: Spending the previous staking transaction output as the input of a new staking transaction with extended timelock (plus funding UTXO to cover fees)
- **Purpose**: Allows stakers to extend their staking period without going through unbonding and restaking
- **Note**: Currently supports extending the timelock duration, not increasing the stake amount

### 8. SLASHED
- **Description**: Penalized state
- **Triggered by**: When staking or unbonding output has been spent through slashing path
- **Sub-States**: Always `TIMELOCK_SLASHING` or `EARLY_UNBONDING_SLASHING` based on which transaction was slashed
- **Possible Flows**:
  - Active → Slashed → Withdrawable → Withdrawn
  - Active → Unbonding → Slashed → Withdrawable → Withdrawn
  - Active → Unbonding → Withdrawable → Slashed → Withdrawable → Withdrawn

## Sub-State Definitions

Sub-states provide additional context about **how** a delegation entered certain states (UNBONDING, WITHDRAWABLE, WITHDRAWN, SLASHED). They track the specific unbonding path taken and whether slashing occurred.

### Sub-State Values

#### 1. TIMELOCK
- **Used in**: UNBONDING, WITHDRAWABLE, WITHDRAWN
- **Meaning**: Delegation reached this state via **natural expiration** (timelock path)
- **Set when**: `EventBTCDelegationExpired` is received from Babylon
- **Example Flow**: `Active → Unbonding(TIMELOCK) → Withdrawable(TIMELOCK) → Withdrawn(TIMELOCK)`

#### 2. EARLY_UNBONDING
- **Used in**: UNBONDING, WITHDRAWABLE, WITHDRAWN
- **Meaning**: Delegation reached this state via **early unbonding request**
- **Set when**: `EventBTCDelegationUnbondedEarly` is received from Babylon or unbonding tx is detected on Bitcoin
- **Example Flow**: `Active → Unbonding(EARLY_UNBONDING) → Withdrawable(EARLY_UNBONDING) → Withdrawn(EARLY_UNBONDING)`

#### 3. TIMELOCK_SLASHING
- **Used in**: SLASHED, WITHDRAWABLE, WITHDRAWN
- **Meaning**: **Staking transaction** was slashed (via slashing path)
- **Set when**: BTC notifier detects staking output spent via slashing path
- **Example Flow**: `Active → Slashed(TIMELOCK_SLASHING) → Withdrawable(TIMELOCK_SLASHING) → Withdrawn(TIMELOCK_SLASHING)`

#### 4. EARLY_UNBONDING_SLASHING
- **Used in**: SLASHED, WITHDRAWABLE, WITHDRAWN
- **Meaning**: **Unbonding transaction** was slashed (via slashing path)
- **Set when**: BTC notifier detects unbonding output spent via slashing path
- **Example Flow**: `Active → Unbonding(EARLY_UNBONDING) → Slashed(EARLY_UNBONDING_SLASHING) → Withdrawable(EARLY_UNBONDING_SLASHING) → Withdrawn(EARLY_UNBONDING_SLASHING)`

### Sub-State Transitions

Sub-states can transition in specific scenarios:

**During Slashing After Expiry**:
- `TIMELOCK` → `TIMELOCK_SLASHING`
  - When staker forgets to withdraw after timelock expiry and delegation gets slashed
- `EARLY_UNBONDING` → `EARLY_UNBONDING_SLASHING`
  - When staker forgets to withdraw after unbonding expiry and delegation gets slashed

## How States Map to API Statuses

The indexer stores delegations with a **State + Sub-State** combination (two separate fields). The Staking API Service transforms these into **flattened statuses** for frontend consumption.

### Mapping Examples

| Indexer State | Indexer Sub-State | API Status |
|--------------|-------------------|------------|
| PENDING | - | PENDING |
| VERIFIED | - | VERIFIED |
| ACTIVE | - | ACTIVE |
| UNBONDING | TIMELOCK | TIMELOCK_UNBONDING |
| UNBONDING | EARLY_UNBONDING | EARLY_UNBONDING |
| WITHDRAWABLE | TIMELOCK | TIMELOCK_WITHDRAWABLE |
| WITHDRAWABLE | EARLY_UNBONDING | EARLY_UNBONDING_WITHDRAWABLE |
| WITHDRAWABLE | TIMELOCK_SLASHING | TIMELOCK_SLASHING_WITHDRAWABLE |
| WITHDRAWABLE | EARLY_UNBONDING_SLASHING | EARLY_UNBONDING_SLASHING_WITHDRAWABLE |
| WITHDRAWN | TIMELOCK | TIMELOCK_WITHDRAWN |
| WITHDRAWN | EARLY_UNBONDING | EARLY_UNBONDING_WITHDRAWN |
| WITHDRAWN | TIMELOCK_SLASHING | TIMELOCK_SLASHING_WITHDRAWN |
| WITHDRAWN | EARLY_UNBONDING_SLASHING | EARLY_UNBONDING_SLASHING_WITHDRAWN |
| SLASHED | TIMELOCK_SLASHING | SLASHED |
| SLASHED | EARLY_UNBONDING_SLASHING | SLASHED |
| EXPANDED | - | EXPANDED |

**For detailed API status definitions and user-facing documentation**, see the [Staking API Service documentation](https://github.com/babylonlabs-io/staking-api-service).