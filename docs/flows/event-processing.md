# Event Processing

The Babylon Staking Indexer processes various events from the Babylon chain to maintain delegation states and finality provider information. Events are processed in order of block height to ensure consistent state transitions.

## Event Types and Their Effects

### Finality Provider Events

1. **EventFinalityProviderCreated**
   - **What**: New finality provider is created in Babylon
   - **Effect in Indexer**: Creates new finality provider record

2. **EventFinalityProviderEdited**
   - **What**: Finality provider details are updated in Babylon
   - **Effect in Indexer**: Updates existing provider details

3. **EventFinalityProviderStatusChange**
   - **What**: Finality provider status is updated in Babylon
   - **Effect in Indexer**: Updates FP active/inactive/jailed/slashed status

### Delegation Events

1. **EventBTCDelegationCreated**
   - **What**: New Expression of Intent (EOI) is created in Babylon
   - **Effect in Indexer**: Creates new delegation record in PENDING state

2. **EventCovenantQuorumReached**
   - **What**: Covenant signatures quorum reached in Babylon
   - **Effect in Indexer**: 
     - Pre-approval flow: Transitions to VERIFIED state
     - Old flow: Transitions to ACTIVE state

3. **EventCovenantSignatureReceived**
   - **What**: Individual covenant member signature received
   - **Effect in Indexer**: Updates signatures in indexer db. 

4. **EventBTCDelegationInclusionProofReceived**
   - **What**: Staking transaction confirmed on Bitcoin
   - **Effect in Indexer**: Transitions to ACTIVE state (in pre-approval flow)

5. **EventBTCDelgationUnbondedEarly**
   - **What**: Staker initiates early unbonding
   - **Effect in Indexer**: Transitions to UNBONDING state

6. **EventBTCDelegationExpired**
   - **What**: Natural expiration at (end height - unbonding time)
   - **Effect in Indexer**: Transitions to UNBONDING state

## Processing Order
- Events are processed sequentially by block height
- Multiple events in the same block are processed in order of appearance (this is very important)