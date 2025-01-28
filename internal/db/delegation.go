package db

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// UpdateOption is a function that modifies update options
type UpdateOption func(*updateOptions)

// updateOptions holds all possible optional parameters
type updateOptions struct {
	subState                *types.DelegationSubState
	bbnHeight               *int64
	btcHeight               *uint32
	stakingSlashingTxInfo   *slashingTxInfo
	unbondingSlashingTxInfo *slashingTxInfo
	stakingStartHeight      *uint32
	stakingEndHeight        *uint32
}

type slashingTxInfo struct {
	txHex          string
	spendingHeight uint32
}

// WithSubState sets the sub-state option
func WithSubState(subState types.DelegationSubState) UpdateOption {
	return func(opts *updateOptions) {
		opts.subState = &subState
	}
}

// WithBbnHeight sets the BBN height option
func WithBbnHeight(height int64) UpdateOption {
	return func(opts *updateOptions) {
		opts.bbnHeight = &height
	}
}

// WithBtcHeight sets the BTC height option
func WithBtcHeight(height uint32) UpdateOption {
	return func(opts *updateOptions) {
		opts.btcHeight = &height
	}
}

// WithStakingStartHeight sets the staking start height option
func WithStakingStartHeight(height uint32) UpdateOption {
	return func(opts *updateOptions) {
		opts.stakingStartHeight = &height
	}
}

// WithStakingEndHeight sets the staking end height option
func WithStakingEndHeight(height uint32) UpdateOption {
	return func(opts *updateOptions) {
		opts.stakingEndHeight = &height
	}
}

// WithStakingSlashingTx sets the staking slashing transaction details
func WithStakingSlashingTx(txHex string, spendingHeight uint32) UpdateOption {
	return func(opts *updateOptions) {
		opts.stakingSlashingTxInfo = &slashingTxInfo{
			txHex:          txHex,
			spendingHeight: spendingHeight,
		}
	}
}

// WithUnbondingSlashingTx sets the unbonding slashing transaction details
func WithUnbondingSlashingTx(txHex string, spendingHeight uint32) UpdateOption {
	return func(opts *updateOptions) {
		opts.unbondingSlashingTxInfo = &slashingTxInfo{
			txHex:          txHex,
			spendingHeight: spendingHeight,
		}
	}
}

func (db *Database) SaveNewBTCDelegation(
	ctx context.Context, delegationDoc *model.BTCDelegationDetails,
) error {
	_, err := db.collection(model.BTCDelegationDetailsCollection).
		InsertOne(ctx, delegationDoc)
	if err != nil {
		var writeErr mongo.WriteException
		if errors.As(err, &writeErr) {
			for _, e := range writeErr.WriteErrors {
				if mongo.IsDuplicateKeyError(e) {
					return &DuplicateKeyError{
						Key:     delegationDoc.StakingTxHashHex,
						Message: "delegation already exists",
					}
				}
			}
		}
		return err
	}
	return nil
}

func (db *Database) UpdateBTCDelegationState(
	ctx context.Context,
	stakingTxHash string,
	qualifiedPreviousStates []types.DelegationState,
	newState types.DelegationState,
	opts ...UpdateOption, // Can pass multiple optional parameters
) error {
	if len(qualifiedPreviousStates) == 0 {
		return fmt.Errorf("qualified previous states array cannot be empty")
	}

	qualifiedStateStrs := make([]string, len(qualifiedPreviousStates))
	for i, state := range qualifiedPreviousStates {
		qualifiedStateStrs[i] = state.String()
	}

	options := &updateOptions{}
	for _, opt := range opts {
		opt(options)
	}

	stateRecord := model.StateRecord{
		State: newState,
	}

	filter := bson.M{
		"_id":   stakingTxHash,
		"state": bson.M{"$in": qualifiedStateStrs},
	}

	updateFields := bson.M{
		"state": newState.String(),
	}

	if options.bbnHeight != nil {
		stateRecord.BbnHeight = *options.bbnHeight
	}

	if options.btcHeight != nil {
		stateRecord.BtcHeight = *options.btcHeight
	}

	if options.subState != nil {
		stateRecord.SubState = *options.subState
		updateFields["sub_state"] = options.subState.String()
	}

	if options.stakingSlashingTxInfo != nil {
		updateFields["slashing_tx.slashing_tx_hex"] = options.stakingSlashingTxInfo.txHex
		updateFields["slashing_tx.spending_height"] = options.stakingSlashingTxInfo.spendingHeight
	}

	if options.unbondingSlashingTxInfo != nil {
		updateFields["slashing_tx.unbonding_slashing_tx_hex"] = options.unbondingSlashingTxInfo.txHex
		updateFields["slashing_tx.spending_height"] = options.unbondingSlashingTxInfo.spendingHeight
	}

	if options.stakingStartHeight != nil {
		updateFields["start_height"] = options.stakingStartHeight
	}

	if options.stakingEndHeight != nil {
		updateFields["end_height"] = options.stakingEndHeight
	}

	update := bson.M{
		"$set": updateFields,
		"$push": bson.M{
			"state_history": stateRecord,
		},
	}

	res := db.collection(model.BTCDelegationDetailsCollection).
		FindOneAndUpdate(ctx, filter, update)

	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return &NotFoundError{
				Key:     stakingTxHash,
				Message: "BTC delegation not found or current state is not qualified states",
			}
		}
		return res.Err()
	}

	return nil
}

func (db *Database) GetBTCDelegationState(
	ctx context.Context, stakingTxHash string,
) (*types.DelegationState, error) {
	delegation, err := db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	if err != nil {
		return nil, err
	}
	return &delegation.State, nil
}

func (db *Database) SaveBTCDelegationUnbondingCovenantSignature(
	ctx context.Context, stakingTxHash string, covenantBtcPkHex string, signatureHex string,
) error {
	filter := bson.M{"_id": stakingTxHash}
	update := bson.M{
		"$push": bson.M{
			"covenant_unbonding_signatures": bson.M{
				"covenant_btc_pk_hex": covenantBtcPkHex,
				"signature_hex":       signatureHex,
			},
		},
	}
	_, err := db.collection(model.BTCDelegationDetailsCollection).
		UpdateOne(ctx, filter, update)

	return err
}

func (db *Database) GetBTCDelegationByStakingTxHash(
	ctx context.Context, stakingTxHash string,
) (*model.BTCDelegationDetails, error) {
	filter := bson.M{"_id": stakingTxHash}

	res := db.collection(model.BTCDelegationDetailsCollection).
		FindOne(ctx, filter)

	var delegationDoc model.BTCDelegationDetails
	err := res.Decode(&delegationDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, &NotFoundError{
				Key:     stakingTxHash,
				Message: "BTC delegation not found when getting by staking tx hash",
			}
		}
		return nil, err
	}

	return &delegationDoc, nil
}

func (db *Database) UpdateDelegationsStateByFinalityProvider(
	ctx context.Context,
	fpBTCPKHex string,
	newState types.DelegationState,
	bbnBlockHeight int64,
) error {
	filter := bson.M{
		"finality_provider_btc_pks_hex": fpBTCPKHex,
	}

	stateRecord := model.StateRecord{
		State:     newState,
		BbnHeight: bbnBlockHeight,
	}

	update := bson.M{
		"$set": bson.M{
			"state": newState.String(),
		},
		"$push": bson.M{
			"state_history": stateRecord,
		},
	}

	result, err := db.collection(model.BTCDelegationDetailsCollection).
		UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update delegations: %w", err)
	}

	log.Printf("Updated %d delegations for finality provider %s to state %s",
		result.ModifiedCount,
		fpBTCPKHex,
		newState.String(),
	)
	return nil
}

func (db *Database) GetDelegationsByFinalityProvider(
	ctx context.Context,
	fpBTCPKHex string,
) ([]*model.BTCDelegationDetails, error) {
	filter := bson.M{
		"finality_provider_btc_pks_hex": fpBTCPKHex,
	}

	cursor, err := db.collection(model.BTCDelegationDetailsCollection).
		Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find delegations: %w", err)
	}
	defer cursor.Close(ctx)

	var delegations []*model.BTCDelegationDetails
	if err := cursor.All(ctx, &delegations); err != nil {
		return nil, fmt.Errorf("failed to decode delegations: %w", err)
	}

	log.Printf("Found %d delegations for finality provider %s",
		len(delegations),
		fpBTCPKHex,
	)
	return delegations, nil
}

func (db *Database) SaveBTCDelegationSlashingTxHex(
	ctx context.Context,
	stakingTxHash string,
	slashingTxHex string,
	spendingHeight uint32,
) error {
	filter := bson.M{"_id": stakingTxHash}
	update := bson.M{
		"$set": bson.M{
			"slashing_tx.slashing_tx_hex": slashingTxHex,
			"slashing_tx.spending_height": spendingHeight,
		},
	}
	result, err := db.collection(model.BTCDelegationDetailsCollection).
		UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return &NotFoundError{
			Key:     stakingTxHash,
			Message: "BTC delegation not found when updating slashing tx hex",
		}
	}

	return nil
}

func (db *Database) SaveBTCDelegationUnbondingSlashingTxHex(
	ctx context.Context,
	stakingTxHash string,
	unbondingSlashingTxHex string,
	spendingHeight uint32,
) error {
	filter := bson.M{"_id": stakingTxHash}
	update := bson.M{
		"$set": bson.M{
			"slashing_tx.unbonding_slashing_tx_hex": unbondingSlashingTxHex,
			"slashing_tx.spending_height":           spendingHeight,
		},
	}
	result, err := db.collection(model.BTCDelegationDetailsCollection).
		UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return &NotFoundError{
			Key:     stakingTxHash,
			Message: "BTC delegation not found when updating unbonding slashing tx hex",
		}
	}

	return nil
}

func (db *Database) GetBTCDelegationsByStates(
	ctx context.Context,
	states []types.DelegationState,
) ([]*model.BTCDelegationDetails, error) {
	// Convert states to a slice of strings
	stateStrings := make([]string, len(states))
	for i, state := range states {
		stateStrings[i] = state.String()
	}

	filter := bson.M{"state": bson.M{"$in": stateStrings}}

	cursor, err := db.collection(model.BTCDelegationDetailsCollection).
		Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var delegations []*model.BTCDelegationDetails
	if err := cursor.All(ctx, &delegations); err != nil {
		return nil, err
	}

	return delegations, nil
}
