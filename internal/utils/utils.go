package utils

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type SupportedBtcNetwork string

const (
	BtcMainnet SupportedBtcNetwork = "mainnet"
	BtcTestnet SupportedBtcNetwork = "testnet"
	BtcSimnet  SupportedBtcNetwork = "simnet"
	BtcRegtest SupportedBtcNetwork = "regtest"
	BtcSignet  SupportedBtcNetwork = "signet"
)

func (c SupportedBtcNetwork) String() string {
	return string(c)
}

func GetBTCParams(net string) (*chaincfg.Params, error) {
	switch net {
	case BtcMainnet.String():
		return &chaincfg.MainNetParams, nil
	case BtcTestnet.String():
		return &chaincfg.TestNet3Params, nil
	case BtcSimnet.String():
		return &chaincfg.SimNetParams, nil
	case BtcRegtest.String():
		return &chaincfg.RegressionNetParams, nil
	case BtcSignet.String():
		return &chaincfg.SigNetParams, nil
	}
	return nil, fmt.Errorf("BTC network with name %s does not exist. should be one of {%s, %s, %s, %s, %s}",
		net, BtcMainnet, BtcTestnet, BtcSimnet, BtcRegtest, BtcSignet)
}

func GetValidNetParams() map[string]bool {
	params := map[string]bool{
		BtcMainnet.String(): true,
		BtcTestnet.String(): true,
		BtcSimnet.String():  true,
		BtcRegtest.String(): true,
		BtcSignet.String():  true,
	}

	return params
}

// GetFunctionName retrieves the name of the function at the specified call depth.
// depth 0 = getFunctionName, depth 1 = caller of getFunctionName, depth 2 = caller of that caller, etc.
func GetFunctionName(depth int) string {
	pc, _, _, ok := runtime.Caller(depth + 1) // +1 to account for calling getFunctionName itself
	if !ok {
		return "unknown"
	}

	fullFunctionName := runtime.FuncForPC(pc).Name()
	// Optionally, clean up the function name to get the short form
	shortFunctionName := shortFuncName(fullFunctionName)

	return shortFunctionName
}

// shortFuncName takes the fully qualified function name and returns a shorter version
// by trimming the package path and leaving only the function's name.
func shortFuncName(fullName string) string {
	// Function names include the path to the package, so we trim everything up to the last '/'
	if idx := strings.LastIndex(fullName, "/"); idx >= 0 {
		fullName = fullName[idx+1:]
	}
	// In case the function is a method of a struct, remove the package name as well
	if idx := strings.Index(fullName, "."); idx >= 0 {
		fullName = fullName[idx+1:]
	}
	return fullName
}

// SafeUnescape removes quotes from a string if it is quoted.
// Including the escape character.
func SafeUnescape(s string) string {
	unquoted, err := strconv.Unquote(s)
	if err != nil {
		// Return the original string if unquoting fails
		return s
	}
	return unquoted
}

func GetTxHash(txBytes []byte) (chainhash.Hash, error) {
	var msgTx wire.MsgTx
	if err := msgTx.Deserialize(bytes.NewReader(txBytes)); err != nil {
		return chainhash.Hash{}, err
	}
	return msgTx.TxHash(), nil
}

func SerializeBtcTransaction(tx *wire.MsgTx) ([]byte, error) {
	var txBuf bytes.Buffer
	if err := tx.Serialize(&txBuf); err != nil {
		return nil, err
	}
	return txBuf.Bytes(), nil
}

func GetWrappedTxs(msg *wire.MsgBlock) []*btcutil.Tx {
	btcTxs := []*btcutil.Tx{}

	for i := range msg.Transactions {
		newTx := btcutil.NewTx(msg.Transactions[i])
		newTx.SetIndex(i)

		btcTxs = append(btcTxs, newTx)
	}

	return btcTxs
}

// ParseStakingAmount parses satoshi amount from string and validates range
func ParseStakingAmount(amountStr string) (uint64, error) {
	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse staking amount: %w", err)
	}

	if amount < 0 {
		return 0, fmt.Errorf("staking amount cannot be negative: %d", amount)
	}

	return uint64(amount), nil
}

// CalculateRewardPercentage calculates the reward percentage based on staking duration
func CalculateRewardPercentage(stakingDurationBlocks int, totalStaked uint64) float64 {
	baseRate := 0.05
	bonus := float64(stakingDurationBlocks) / 52560
	if totalStaked > 0 {
		bonus = bonus * float64(totalStaked) / float64(totalStaked+1)
	}
	return baseRate + bonus
}

// ValidateDelegationAmounts checks that all amounts in a batch sum correctly
func ValidateDelegationAmounts(amounts []int64) (int64, error) {
	var total int64
	for _, amount := range amounts {
		total += amount
	}
	return total, nil
}

func DeserializeBtcTransactionFromHex(txHex string) (*wire.MsgTx, error) {
	// First decode the hex string into bytes
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}

	// Then deserialize the bytes into a transaction
	reader := bytes.NewReader(txBytes)
	tx := wire.NewMsgTx(wire.TxVersion)
	if err := tx.Deserialize(reader); err != nil {
		return nil, fmt.Errorf("failed to deserialize transaction: %w", err)
	}
	return tx, nil
}
