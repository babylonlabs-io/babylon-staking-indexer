package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbntypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gogo/protobuf/proto"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func DumpEventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "dump-events",
		Run: dumpEvents,
	}

	cmd.Flags().Int("workers", 0, "Number of workers to process records")

	return cmd
}

func dumpEvents(cmd *cobra.Command, args []string) {
	err := dumpEventsE(cmd, args)
	// because of current architecture we need to stop execution of the program
	// otherwise existing main logic will be called
	if err != nil {
		log.Err(err).Msg("Failed to dump events")
		os.Exit(1)
	}

	os.Exit(0)
}

func dumpEventsE(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancelCause(cmd.Context())
	defer cancel(nil)

	cfg, err := config.New(GetConfigPath())
	if err != nil {
		return err
	}

	cl, err := bbnclient.NewBBNClient(&cfg.BBN)
	if err != nil {
		return err
	}

	blockHeight, err := cl.GetLatestBlockNumber(ctx)
	if err != nil {
		return err
	}
	defer func() {
		fmt.Println("Last block height:", blockHeight)
	}()

	var wg sync.WaitGroup
	// todo workers
	workers := runtime.GOMAXPROCS(0)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				if ctx.Err() != nil {
					return
				}

				height := atomic.AddInt64(&blockHeight, -1)
				if height%100 == 0 {
					fmt.Printf("Height %d\n", height)
				}

				block, err := cl.GetBlockResults(ctx, &height)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}

					cancel(err)
					return
				}

				for _, txResult := range block.TxsResults {
					for _, event := range txResult.Events {
						logEvent(block.Height, event)
					}
				}
				// Append finalize-block-level events
				for _, event := range block.FinalizeBlockEvents {
					logEvent(block.Height, event)
				}

				// make configurable
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(10)))
			}
		}()

	}

	wg.Wait()
	return context.Cause(ctx)
}

func logEvent(blockNumber int64, event abcitypes.Event) {
	eventType := types.EventType(event.Type)

	logError := func(err error) {
		fmt.Printf("Error [block %d]: %v\n", blockNumber, err)
	}
	logData := func(v any) {
		buff, err := json.Marshal(v)
		if err != nil {
			logError(err)
			return
		}

		fmt.Printf("Event [block %d]: %s %s\n", blockNumber, eventType, string(buff))
	}

	switch eventType {
	case types.EventFinalityProviderCreatedType:
		_, err := parseEvent[*bbntypes.EventFinalityProviderCreated](
			eventType, event,
		)
		if err != nil {
			logError(err)
			return
		}

		// logData(newFP)
	case types.EventFinalityProviderEditedType:
		fpEdited, err := parseEvent[*bbntypes.EventFinalityProviderEdited](
			eventType, event,
		)
		if err != nil {
			logError(err)
			return
		}

		logData(fpEdited)
	case types.EventFinalityProviderStatusChange:
		fpStatusChanged, err := parseEvent[*bbntypes.EventFinalityProviderStatusChange](
			eventType, event,
		)
		if err != nil {
			logError(err)
			return
		}

		logData(fpStatusChanged)
	}
}

func parseEvent[T proto.Message](
	expectedType types.EventType,
	event abcitypes.Event,
) (T, error) {
	var result T

	// Check if the event type matches the expected type
	if types.EventType(event.Type) != expectedType {
		return result, fmt.Errorf(
			"unexpected event type: %s received when processing %s",
			event.Type,
			expectedType,
		)
	}

	// Check if the event has attributes
	if len(event.Attributes) == 0 {
		return result, fmt.Errorf(
			"no attributes found in the %s event",
			expectedType,
		)
	}

	// Sanitize the event attributes before parsing
	sanitizedEvent := sanitizeEvent(event)

	// Use the SDK's ParseTypedEvent function
	protoMsg, err := sdk.ParseTypedEvent(sanitizedEvent)
	if err != nil {
		return result, fmt.Errorf("failed to parse typed event: %w", err)
	}

	// Type assertion to ensure we have the correct concrete type
	concreteMsg, ok := protoMsg.(T)
	if !ok {
		return result, fmt.Errorf("parsed event type %T does not match expected type %T", protoMsg, result)
	}

	return concreteMsg, nil
}

func sanitizeEvent(event abcitypes.Event) abcitypes.Event {
	sanitizedAttrs := make([]abcitypes.EventAttribute, len(event.Attributes))
	for i, attr := range event.Attributes {
		// Remove any extra quotes and ensure proper JSON formatting
		value := strings.Trim(attr.Value, "\"")
		// If the value isn't already a JSON value (object, array, or quoted string),
		// wrap it in quotes
		if !strings.HasPrefix(value, "{") && !strings.HasPrefix(value, "[") {
			value = fmt.Sprintf("\"%s\"", value)
		}

		sanitizedAttrs[i] = abcitypes.EventAttribute{
			Key:   attr.Key,
			Value: value,
			Index: attr.Index,
		}
	}

	return abcitypes.Event{
		Type:       event.Type,
		Attributes: sanitizedAttrs,
	}
}
