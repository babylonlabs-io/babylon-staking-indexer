package db

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (db *Database) SaveNewFinalityProvider(
	ctx context.Context, fpDoc *model.FinalityProviderDetails,
) error {
	_, err := db.collection(model.FinalityProviderDetailsCollection).
		InsertOne(ctx, fpDoc)
	if err != nil {
		var writeErr mongo.WriteException
		if errors.As(err, &writeErr) {
			for _, e := range writeErr.WriteErrors {
				if mongo.IsDuplicateKeyError(e) {
					return &DuplicateKeyError{
						Key:     fpDoc.BtcPk,
						Message: "finality provider already exists",
					}
				}
			}
		}
		return err
	}
	return nil
}

func (db *Database) UpdateFinalityProviderDetailsFromEvent(
	ctx context.Context, detailsToUpdate *model.FinalityProviderDetails,
) error {
	updateFields := bson.M{}

	// Only add fields to updateFields if they are not empty
	if detailsToUpdate.Commission != "" {
		updateFields["commission"] = detailsToUpdate.Commission
	}
	if detailsToUpdate.Description.Moniker != "" {
		updateFields["description.moniker"] = detailsToUpdate.Description.Moniker
	}
	if detailsToUpdate.Description.Identity != "" {
		updateFields["description.identity"] = detailsToUpdate.Description.Identity
	}
	if detailsToUpdate.Description.Website != "" {
		updateFields["description.website"] = detailsToUpdate.Description.Website
	}
	if detailsToUpdate.Description.SecurityContact != "" {
		updateFields["description.security_contact"] = detailsToUpdate.Description.SecurityContact
	}
	if detailsToUpdate.Description.Details != "" {
		updateFields["description.details"] = detailsToUpdate.Description.Details
	}

	// Perform the update only if there are fields to update
	if len(updateFields) > 0 {
		res, err := db.collection(model.FinalityProviderDetailsCollection).
			UpdateOne(
				ctx, bson.M{"_id": detailsToUpdate.BtcPk}, bson.M{"$set": updateFields},
			)

		// Check if the document was found and updated
		if err != nil {
			return err
		}
		if res.MatchedCount == 0 {
			return &NotFoundError{
				Key:     detailsToUpdate.BtcPk,
				Message: "finality provider not found when updating details",
			}
		}
	}

	return nil
}

func (db *Database) UpdateFinalityProviderState(
	ctx context.Context, btcPk string, newState string,
) error {
	filter := map[string]string{"_id": btcPk}
	update := map[string]interface{}{"$set": map[string]string{"state": newState}}

	// Perform the find and update
	res := db.collection(model.FinalityProviderDetailsCollection).
		FindOneAndUpdate(ctx, filter, update)

	// Check if the document was found
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return &NotFoundError{
				Key:     btcPk,
				Message: "finality provider not found when updating state",
			}
		}
		return res.Err()
	}

	return nil
}

func (db *Database) GetFinalityProviderByBtcPk(
	ctx context.Context, btcPk string,
) (*model.FinalityProviderDetails, error) {
	filter := map[string]interface{}{"_id": btcPk}
	res := db.collection(model.FinalityProviderDetailsCollection).
		FindOne(ctx, filter)

	var fpDoc model.FinalityProviderDetails
	err := res.Decode(&fpDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, &NotFoundError{
				Key:     btcPk,
				Message: "finality provider not found when getting by btc public key",
			}
		}
		return nil, err
	}

	return &fpDoc, nil
}
