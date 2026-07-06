package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// buildHeadlessConfig assembles a NodeConfig from environment variables so a
// node can run non-interactively (Docker, a website's server, CI). It is used
// on first run in --headless mode; on later runs the saved config is reused.
//
// Env vars (all optional):
//
//	TRUTHCHAIN_NETWORK        local | testnet | mainnet   (default local)
//	TRUTHCHAIN_DB_PATH        default truthchain.db
//	TRUTHCHAIN_WALLET_PATH    default wallet.json
//	TRUTHCHAIN_API_PORT       default 8080
//	TRUTHCHAIN_MESH_PORT      default 9876
//	TRUTHCHAIN_MODES          csv of api,mesh,mining,beacon (default api,mesh,mining)
//	TRUTHCHAIN_DOMAIN         beacon domain (required if beacon mode)
//	TRUTHCHAIN_PUBLIC_API     true|false  (bind API to 0.0.0.0)
//	TRUTHCHAIN_API_RATE_LIMIT requests/min per IP
//	TRUTHCHAIN_FAUCET         true|false
//	TRUTHCHAIN_FAUCET_AMOUNT  characters per claim
//	TRUTHCHAIN_FAUCET_COOLDOWN_SEC / TRUTHCHAIN_FAUCET_DAILY_CAP
func buildHeadlessConfig() *NodeConfig {
	networkID, postThreshold := "truthchain-local", 2
	switch strings.ToLower(envStr("TRUTHCHAIN_NETWORK", "local")) {
	case "mainnet":
		networkID, postThreshold = "truthchain-mainnet", 5
	case "testnet":
		networkID, postThreshold = "truthchain-testnet", 3
	}

	modes := strings.ToLower(envStr("TRUTHCHAIN_MODES", "api,mesh,mining"))
	hasMode := func(m string) bool { return strings.Contains(","+modes+",", ","+m+",") }

	cfg := &NodeConfig{
		DBPath:            envStr("TRUTHCHAIN_DB_PATH", "truthchain.db"),
		WalletPath:        envStr("TRUTHCHAIN_WALLET_PATH", "wallet.json"),
		APIPort:           envInt("TRUTHCHAIN_API_PORT", 8080),
		MeshPort:          envInt("TRUTHCHAIN_MESH_PORT", 9876),
		PostThreshold:     postThreshold,
		NetworkID:         networkID,
		APIMode:           hasMode("api"),
		MeshMode:          hasMode("mesh"),
		MiningMode:        hasMode("mining"),
		BeaconMode:        hasMode("beacon"),
		Domain:            envStr("TRUTHCHAIN_DOMAIN", ""),
		PublicAPI:         envBool("TRUTHCHAIN_PUBLIC_API", false),
		APIRateLimit:      envInt("TRUTHCHAIN_API_RATE_LIMIT", 0),
		FaucetEnabled:     envBool("TRUTHCHAIN_FAUCET", false),
		FaucetAmount:      envInt("TRUTHCHAIN_FAUCET_AMOUNT", 0),
		FaucetCooldownSec: envInt("TRUTHCHAIN_FAUCET_COOLDOWN_SEC", 0),
		FaucetDailyCap:    envInt("TRUTHCHAIN_FAUCET_DAILY_CAP", 0),
	}

	log.Printf("[Headless] network=%s api=:%d mesh=:%d modes=%s publicAPI=%t faucet=%t",
		cfg.NetworkID, cfg.APIPort, cfg.MeshPort, modes, cfg.PublicAPI, cfg.FaucetEnabled)
	return cfg
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Printf("[Headless] invalid int for %s=%q, using default %d", key, v, def)
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
		log.Printf("[Headless] invalid bool for %s=%q, using default %t", key, v, def)
	}
	return def
}
