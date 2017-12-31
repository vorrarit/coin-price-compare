package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// type BxResponse struct {
// 	Pairs map[string]BxPair
// }

type BxPair struct {
	PairID            int     `json:"pairing_id"`
	PrimaryCurrency   string  `json:"primary_currency"`
	SecondaryCurrency string  `json:"secondary_currency"`
	LastPrice         float64 `json:"last_price"`
}

type BfPair struct {
	LastPrice string `json:"last_price"`
}

type BotResultDataDetail struct {
	Rate string `json:"rate"`
}
type BotResultData struct {
	DataDetail []BotResultDataDetail `json:"data_detail"`
}
type BotResult struct {
	Data BotResultData `json:"data"`
}
type BotDailyRate struct {
	Result BotResult `json:"result"`
}

func GetExchangeRate(botChannel chan map[string]float64) {
	defer wg.Done()

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://iapi.bot.or.th/Stat/Stat-ReferenceRate/DAILY_REF_RATE_V1/?start_period=2017-12-26&end_period=2017-12-26", nil)
	req.Header.Add("api-key", "U9G1L457H6DCugT7VmBaEacbHV9RX0PySO05cYaGsm")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	var botResponse BotDailyRate
	json.Unmarshal(body, &botResponse)

	rate, _ := strconv.ParseFloat(botResponse.Result.Data.DataDetail[0].Rate, 64)
	pairs := make(map[string]float64)
	pairs["USD_THB"] = rate
	pairs["THB_USD"] = 1 / rate

	botChannel <- pairs
}

func GetBxPrices(bxChannel chan map[string]float64) {
	defer wg.Done()

	resp, err := http.Get("https://bx.in.th/api/")
	if err != nil {
		fmt.Println(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	var bxResponse map[string]BxPair
	json.Unmarshal(body, &bxResponse)

	pairs := make(map[string]float64)
	for _, bxPair := range bxResponse {
		if bxPair.PrimaryCurrency == "THB" {
			pairs[bxPair.SecondaryCurrency+"_"+bxPair.PrimaryCurrency] = bxPair.LastPrice
			pairs[bxPair.PrimaryCurrency+"_"+bxPair.SecondaryCurrency] = 1 / bxPair.LastPrice
			pairs[bxPair.SecondaryCurrency+"_USD"] = bxPair.LastPrice / botRates["USD_THB"]
			pairs["USD_"+bxPair.SecondaryCurrency] = (1 / bxPair.LastPrice) / botRates["USD_THB"]
		}
	}

	bxChannel <- pairs
}

func GetBfPair(bfChannel chan map[string]float64, symbol string) {
	defer subWg.Done()

	pairs := make(map[string]float64)

	resp, err := http.Get("https://api.bitfinex.com/v1/pubticker/" + symbol)
	if err != nil {
		fmt.Println(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	var bfPair BfPair
	json.Unmarshal(body, &bfPair)

	secondaryCurrency := strings.ToUpper(symbol[0:3])
	primaryCurrency := strings.ToUpper(symbol[3:])
	if secondaryCurrency == "DSH" {
		secondaryCurrency = "DAS"
	}

	rate, _ := strconv.ParseFloat(bfPair.LastPrice, 64)
	pairs[secondaryCurrency+"_"+primaryCurrency] = rate
	pairs[primaryCurrency+"_"+secondaryCurrency] = 1 / rate

	bfChannel <- pairs
}

func GetBfPrices(bfChannel chan map[string]float64) {
	defer wg.Done()

	symbols := [...]string{"btcusd", "ethusd", "bchusd", "dshusd", "ltcusd", "xrpusd", "omgusd"}
	bfSubChannel := make(chan map[string]float64, 10)
	for _, symbol := range symbols {
		subWg.Add(1)
		go GetBfPair(bfSubChannel, symbol)
	}
	subWg.Wait()
	close(bfSubChannel)

	pairs := make(map[string]float64)
	for items := range bfSubChannel {
		for itemKey, itemValue := range items {
			pairs[itemKey] = itemValue
		}
	}
	bfChannel <- pairs
}

func printPriceDiff(firstMarket map[string]float64, firstMarketBaseCurrency string, secondMarket map[string]float64, secondMarketBaseCurrency string, preferCurrency string) {
	// bfWithdrawFees := map[string]float64{"BTC": 0.0008, "BCH": 0.0001, "ETH": 0.01, "LTC": 0.001, "OMG": 0.1, "ZEC": 0.001, "DAS": 0.01, "XRP": 0.02}
	// bxWithdrawFees := map[string]float64{"BTC": 0.002, "BCH": 0.0001, "ETH": 0.005, "LTC": 0.005, "OMG": 0.2, "ZEC": 0.001, "DAS": 0.005, "XRP": 0.01}
	// var bxTradingFee, bfTradingFee float64
	// bxTradingFee = 0.25 / 100
	// bfTradingFee = 0.1 / 100

	coins := []string{"BTC", "BCH", "ETH", "LTC", "OMG", "DAS", "XRP"}
	for _, coin := range coins {
		firstMarketPrice := firstMarket[coin+"_"+firstMarketBaseCurrency]
		firstMarketPricePC := firstMarketPrice
		if firstMarketBaseCurrency != preferCurrency {
			firstMarketPricePC = firstMarketPrice * botRates[firstMarketBaseCurrency+"_"+preferCurrency]
		}
		secondMarketPrice := secondMarket[coin+"_"+secondMarketBaseCurrency]
		secondMarketPricePC := secondMarketPrice
		if secondMarketBaseCurrency != preferCurrency {
			secondMarketPricePC = secondMarketPrice * botRates[secondMarketBaseCurrency+"_"+preferCurrency]
		}

		fmt.Printf("%s, %s %f, %s %f, %s %f, %s %f\n", coin, firstMarketBaseCurrency, firstMarketPrice, secondMarketBaseCurrency, secondMarketPrice, preferCurrency, firstMarketPricePC, preferCurrency, secondMarketPricePC)
	}
}

func printTransfer(inputAmount float64,
	firstMarketLabel string, firstMarketTradingFee float64, firstMarketPrices map[string]float64, firstMarketWithdrawFees map[string]float64, firstMarketBase string, firstMarketCC string,
	secondMarketLabel string, secondMarketTradingFee float64, secondMarketPrices map[string]float64, secondMarketWithdrawFees map[string]float64, secondMarketBase string, secondMarketCC string,
	thirdMarketLabel string, thirdMarketTradingFee float64, thirdMarketPrices map[string]float64, thirdMarketWithdrawFees map[string]float64, thirdMarketBase string, thirdMarketCC string,
	preferCurrency string) {

	inputAmountAfterFee := inputAmount * (1 - firstMarketTradingFee)
	firstMarketInputCC := inputAmountAfterFee / firstMarketPrices[firstMarketCC+"_"+firstMarketBase]

	secondMarketInputCC := firstMarketInputCC - firstMarketWithdrawFees[firstMarketCC]
	secondMarketOutput := secondMarketInputCC * secondMarketPrices[firstMarketCC+"_"+secondMarketBase]

	fmt.Printf("%s %s -> %s, %s %s -> %s, %s %s -> %s\n", firstMarketLabel, firstMarketBase, firstMarketCC, secondMarketLabel, firstMarketCC, secondMarketCC, thirdMarketLabel, secondMarketCC, thirdMarketCC)
	fmt.Printf("Input Amount %s %f\n", firstMarketBase, inputAmount)
	fmt.Printf("%sinputCC: %.8f, %sinputCC: %.8f -> %s %f ", firstMarketLabel, firstMarketInputCC, secondMarketLabel, secondMarketInputCC, secondMarketBase, secondMarketOutput)
	if secondMarketBase != preferCurrency {
		fmt.Printf("(%s %.2f)", preferCurrency, secondMarketOutput*botRates[secondMarketBase+"_"+preferCurrency])
	}
	fmt.Println("\n")

}

var botRates map[string]float64
var wg sync.WaitGroup
var subWg sync.WaitGroup

func main() {

	botChannel := make(chan map[string]float64, 1)
	bxChannel := make(chan map[string]float64, 1)
	bfChannel := make(chan map[string]float64, 1)

	wg.Add(1)
	go GetExchangeRate(botChannel)

	wg.Add(1)
	go GetBxPrices(bxChannel)

	wg.Add(1)
	go GetBfPrices(bfChannel)

	wg.Wait()

	close(botChannel)
	close(bxChannel)
	close(bfChannel)

	botRates = <-botChannel
	bxPrices := <-bxChannel
	bfPrices := <-bfChannel
	// bxPrices := GetBxPrices()
	// bfPrices := GetBfPrices()

	fmt.Println(bfPrices)

	printPriceDiff(bxPrices, "THB", bfPrices, "USD", "THB")

	bfWithdrawFees := map[string]float64{"BTC": 0.0008, "BCH": 0.0001, "ETH": 0.01, "LTC": 0.001, "OMG": 0.1, "ZEC": 0.001, "DAS": 0.01, "XRP": 0.02}
	bxWithdrawFees := map[string]float64{"BTC": 0.002, "BCH": 0.0001, "ETH": 0.005, "LTC": 0.005, "OMG": 0.2, "ZEC": 0.001, "DAS": 0.005, "XRP": 0.01}
	var bxTradingFee, bfTradingFee float64
	bxTradingFee = 0.25 / 100
	bfTradingFee = 0.1 / 100

	coins := []string{"BTC", "BCH", "ETH", "LTC", "OMG", "DAS", "XRP"}
	for _, coin := range coins {
		printTransfer(100000,
			"BX", bxTradingFee, bxPrices, bxWithdrawFees, "THB", coin,
			"BF", bfTradingFee, bfPrices, bfWithdrawFees, "USD", "XRP",
			"BX", bxTradingFee, bxPrices, bxWithdrawFees, "THB", "THB", "THB")
	}

	for _, coin := range coins {
		printTransfer(3000,
			"BF", bfTradingFee, bfPrices, bfWithdrawFees, "USD", coin,
			"BX", bxTradingFee, bxPrices, bxWithdrawFees, "THB", "XRP",
			"BF", bfTradingFee, bfPrices, bfWithdrawFees, "USD", "USD", "USD")
	}
}
