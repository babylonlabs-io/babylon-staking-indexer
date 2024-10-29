package utils

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"strings"

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
		net, BtcMainnet.String(), BtcTestnet.String(), BtcSimnet.String(), BtcRegtest.String(), BtcSignet.String())
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

// Contains checks if a slice contains a specific element
func Contains[T comparable](slice []T, item T) bool {
	for _, elem := range slice {
		if elem == item {
			return true
		}
	}
	return false
}

func GetTxHash(txBytes []byte) (chainhash.Hash, error) {
	var msgTx wire.MsgTx
	if err := msgTx.Deserialize(bytes.NewReader(txBytes)); err != nil {
		return chainhash.Hash{}, err
	}
	return msgTx.TxHash(), nil
}
