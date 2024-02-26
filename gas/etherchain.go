package gas

import (
	"fmt"
	"io"
	"math/big"
	"net/http"

	"github.com/goccy/go-json"
)

const gasNowUrl string = "https://beaconcha.in/api/v1/execution/gasnow"

// Standard response
type gasNowResponse struct {
	Data struct {
		Rapid    uint64  `json:"rapid"`
		Fast     uint64  `json:"fast"`
		Standard uint64  `json:"standard"`
		Slow     uint64  `json:"slow"`
		PriceUSD float64 `json:"priceUSD"`
	} `json:"data"`
}

type EtherchainGasFeeSuggestion struct {
	RapidWei  *big.Int
	RapidTime string

	FastWei  *big.Int
	FastTime string

	StandardWei  *big.Int
	StandardTime string

	SlowWei  *big.Int
	SlowTime string

	EthUsd float64
}

// Get gas prices
func GetEtherchainGasPrices() (EtherchainGasFeeSuggestion, error) {
	// Send request
	response, err := http.Get(gasNowUrl)
	if err != nil {
		return EtherchainGasFeeSuggestion{}, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	// Check the response code
	if response.StatusCode != http.StatusOK {
		return EtherchainGasFeeSuggestion{}, fmt.Errorf("request failed with code %d", response.StatusCode)
	}

	// Get response
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return EtherchainGasFeeSuggestion{}, err
	}

	// Deserialize response
	var gnResponse gasNowResponse
	if err := json.Unmarshal(body, &gnResponse); err != nil {
		return EtherchainGasFeeSuggestion{}, fmt.Errorf("error getting Etherchain Gas Now response: %w", err)
	}

	suggestion := EtherchainGasFeeSuggestion{
		RapidWei:  big.NewInt(0).SetUint64(gnResponse.Data.Rapid),
		RapidTime: "15 Seconds",

		FastWei:  big.NewInt(0).SetUint64(gnResponse.Data.Fast),
		FastTime: "1 Minute",

		StandardWei:  big.NewInt(0).SetUint64(gnResponse.Data.Standard),
		StandardTime: "3 Minutes",

		SlowWei:  big.NewInt(0).SetUint64(gnResponse.Data.Slow),
		SlowTime: ">10 Minutes",

		EthUsd: gnResponse.Data.PriceUSD,
	}

	// Return
	return suggestion, nil
}
