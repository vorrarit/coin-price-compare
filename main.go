package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// type BxResponse struct {
// 	Pairs map[string]BxPair
// }

type BxPair struct {
	PairId            int     `json:"pairing_id"`
	PrimaryCurrency   string  `json:"primary_currency"`
	SecondaryCurrency string  `json:"secondary_currency"`
	LastPrice         float64 `json:"last_price"`
}

func main() {
	var bxResponse map[string]BxPair

	resp, err := http.Get("https://bx.in.th/api/")
	if err != nil {
		fmt.Println(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	json.Unmarshal(body, &bxResponse)

	for _, bxPair := range bxResponse {
		fmt.Println(bxPair.PrimaryCurrency)
	}
}
