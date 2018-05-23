package bitbitx

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/nntaoli-project/GoEx"
)

/*
1. 服务端以JSON格式返回结果给客户端，结构如下：
单条数据：
{"returnCode":"0","returnMsg":"","returnParams":{…}}

多条数据：
{"returnCode":"0","returnMsg":"","returnParams":{"rows":[…],"page":1, "records":8, "total":1}}

returnCode：成功失败状态， 0：成功
returnMsg：失败时返回失败原因
returnParams：返回的数据
rows：返回的数组
records：数据总数
total：页码总数
page：当前页码

2. 接口要求使用post方法请求；
3. 接口请求参数使用DES3加密后再base64加密
4. 接口返回结果使用base64解密后再DES3解密
5. 需要登录的方法，在header里传token以及uuid参数；
*/

const (
	EXCHANGE_NAME = "bitbitx.com"

	API_BASE_URL = "http://114.115.202.30:82/"
	API_METHOD   = "api_out/clientService"
	SYMBOLS_URI  = "public/symbol"
	TICKER_URI   = "public/ticker/"
	BALANCE_URI  = "account/balance"
	ORDER_URI    = "order"
	DEPTH_URI    = "public/orderbook"
	TRADES_URI   = "public/trades"
	KLINE_URI    = "public/candles"

	CODE_OK = "0"
)

var (
	YCC     = goex.Currency{"YCC", "Yuan Chain New"}
	BTC     = goex.Currency{"BTC", "Bitcoin"}
	YCC_BTC = goex.CurrencyPair{YCC, BTC}

	token string
	uuid  string
)

type BitBitx struct {
	des3Key    string
	httpClient *http.Client
}

func New(client *http.Client, des3Key string) *BitBitx {
	return &BitBitx{des3Key, client}
}

func (btx *BitBitx) GetExchangeName() string {
	return EXCHANGE_NAME
}

func (btx *BitBitx) Register(mobile, email, name, pwd, payPwd string) error {
	params := fmt.Sprintf(`{"serviceId":"%v","params":{"mobile":"%v","email":"%v","name":"%v","password":"%v","pay_pwd":"%v"}}`,
		"dna_register", mobile, email, name, pwd, payPwd)
	_, err := btx.doRequest(params)
	return err
}

func (btx *BitBitx) Login(user, pwd string) error {
	params := fmt.Sprintf(`{"serviceId":"%v","params":{"username":"%v","password":"%v"}}`, "dna_personLogin", user, pwd)
	bytes, err := btx.doRequest(params)
	if err != nil {
		return err
	}
	var resp map[string]interface{}
	err = json.Unmarshal(bytes, &resp)
	if err != nil {
		return errors.New("fail to unmarshal to json maybe caused by invalid DES3 key")
	}

	if resp["returnCode"].(string) != CODE_OK {
		return errors.New(resp["returnMsg"].(string))
	}

	returnParams := resp["returnParams"].(map[string]interface{})
	uuid = fmt.Sprintf("%v", goex.ToUint64(returnParams["f01"]))
	token = returnParams["token"].(string)

	fmt.Printf("uuid: %v, token: %v\n", uuid, token)
	return nil
}

/*
func (btx *BitBitx) GetSymbols() ([]goex.CurrencyPair, error) {
	resp := []map[string]interface{}{}
	err := btx.doRequest("GET", SYMBOLS_URI, &resp)
	if err != nil {
		return nil, err
	}

	pairs := []goex.CurrencyPair{}
	for _, e := range resp {
		one := goex.CurrencyPair{
			CurrencyA: goex.Currency{e["baseCurrency"].(string), ""},
			CurrencyB: goex.Currency{e["quoteCurrency"].(string), ""},
		}
		pairs = append(pairs, one)
	}
	return pairs, nil
}

func (btx *BitBitx) GetTicker(currency goex.CurrencyPair) (*goex.Ticker, error) {
	currency = btx.adaptCurrencyPair(currency)
	curr := currency.ToSymbol("")
	tickerUri := API_BASE_URL + API_METHOD + TICKER_URI + curr
	bodyDataMap, err := goex.HttpGet(btx.httpClient, tickerUri)
	if err != nil {
		return nil, err
	}

	if result, isok := bodyDataMap["error"].(map[string]interface{}); isok == true {
		return nil, errors.New(result["message"].(string) + ", " + result["description"].(string))
	}

	tickerMap := bodyDataMap
	var ticker goex.Ticker

	timestamp := time.Now().Unix()
	ticker.Date = uint64(timestamp)
	ticker.Last = goex.ToFloat64(tickerMap["last"])
	ticker.Buy = goex.ToFloat64(tickerMap["bid"])
	ticker.Sell = goex.ToFloat64(tickerMap["ask"])
	ticker.Low = goex.ToFloat64(tickerMap["low"])
	ticker.High = goex.ToFloat64(tickerMap["high"])
	ticker.Vol = goex.ToFloat64(tickerMap["volume"])

	return &ticker, nil
}

func (btx *BitBitx) placeOrder(ty goex.TradeSide, amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	postData := url.Values{}
	postData.Set("symbol", currency.ToSymbol(""))
	var side string
	var orderType string
	switch ty {
	case goex.BUY:
		side = "buy"
		orderType = "limit"
	case goex.BUY_MARKET:
		side = "buy"
		orderType = "market"
	case goex.SELL:
		side = "sell"
		orderType = "limit"
	case goex.SELL_MARKET:
		side = "sell"
		orderType = "market"
	default:
		panic(ty)
	}
	postData.Set("side", side)
	postData.Set("type", orderType)
	postData.Set("quantity", amount)
	if orderType == "limit" {
		postData.Set("price", price)
	}

	reqUrl := API_BASE_URL + API_METHOD + ORDER_URI
	headers := make(map[string]string)
	headers["Content-type"] = "application/x-www-form-urlencoded"
	headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(btx.des3Key+":"+btx.des3Key))

	bytes, err := goex.HttpPostForm3(btx.httpClient, reqUrl, postData.Encode(), headers)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]interface{})
	err = json.Unmarshal(bytes, &resp)
	if err != nil {
		return nil, err
	}

	if errObj, ok := resp["error"]; ok {
		log.Println(errObj)
		return nil, errors.New(errObj.(map[string]string)["message"])
	}

	return toOrder(resp), nil
}

func (btx *BitBitx) LimitBuy(amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	return btx.placeOrder(goex.BUY, amount, price, currency)
}

func (btx *BitBitx) LimitSell(amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	return btx.placeOrder(goex.SELL, amount, price, currency)
}

func (btx *BitBitx) MarketBuy(amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	return btx.placeOrder(goex.BUY_MARKET, amount, price, currency)
}

func (btx *BitBitx) MarketSell(amount, price string, currency goex.CurrencyPair) (*goex.Order, error) {
	return btx.placeOrder(goex.SELL_MARKET, amount, price, currency)
}

func (btx *BitBitx) CancelOrder(orderId string, currency goex.CurrencyPair) (bool, error) {
	postData := url.Values{}
	reqUrl := API_BASE_URL + API_METHOD + ORDER_URI + "/" + orderId
	headers := make(map[string]string)
	headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(btx.des3Key+":"+btx.des3Key))
	bytes, err := goex.HttpDeleteForm(btx.httpClient, reqUrl, postData, headers)
	if err != nil {
		return false, err
	}

	var resp map[string]interface{}
	err = json.Unmarshal(bytes, &resp)
	if err != nil {
		return false, err
	}

	if errObj, ok := resp["error"]; ok {
		log.Println(errObj)
		return false, errors.New(errObj.(map[string]string)["message"])
	}

	return true, nil
}

func (btx *BitBitx) GetOneOrder(orderId string, currency goex.CurrencyPair) (*goex.Order, error) {
	resp := make(map[string]interface{})
	err := btx.doRequest("GET", ORDER_URI+"/"+orderId, &resp)
	if err != nil {
		return nil, err
	}

	if errObj, ok := resp["error"]; ok {
		return nil, errors.New(errObj.(map[string]string)["message"])
	}

	return toOrder(resp), nil
}

func (btx *BitBitx) GetUnfinishOrders(currency goex.CurrencyPair) ([]goex.Order, error) {
	params := url.Values{}
	params.Set("symbol", currency.ToSymbol(""))
	resp := []map[string]interface{}{}
	err := btx.doRequest("GET", ORDER_URI+"?"+params.Encode(), &resp)
	if err != nil {
		return nil, err
	}

	// TODO error

	orders := []goex.Order{}
	for _, e := range resp {
		o := toOrder(e)
		if o.Status == goex.ORDER_UNFINISH || o.Status == goex.ORDER_PART_FINISH {
			orders = append(orders, *o)
		}
	}
	return orders, nil
}

func (btx *BitBitx) GetOrderHistorys(currency goex.CurrencyPair, currentPage, pageSize int) ([]goex.Order, error) {
	params := url.Values{}
	params.Set("symbol", currency.ToSymbol(""))
	resp := []map[string]interface{}{}
	err := btx.doRequest("GET", ORDER_URI+"?"+params.Encode(), &resp)
	if err != nil {
		return nil, err
	}

	// TODO error

	orders := []goex.Order{}
	for _, e := range resp {
		o := toOrder(e)
		orders = append(orders, *o)
	}
	return orders, nil
}

func (btx *BitBitx) GetAccount() (*goex.Account, error) {
	var ret []interface{}
	err := btx.doRequest("GET", BALANCE_URI, &ret)
	if err != nil {
		return nil, err
	}

	acc := new(goex.Account)
	acc.SubAccounts = make(map[goex.Currency]goex.SubAccount, 1)

	for _, v := range ret {
		vv := v.(map[string]interface{})
		currency := goex.NewCurrency(vv["currency"].(string), "")
		acc.SubAccounts[currency] = goex.SubAccount{
			Currency:     currency,
			Amount:       goex.ToFloat64(vv["available"]),
			ForzenAmount: goex.ToFloat64(vv["reserved"])}
	}

	return acc, nil
}

func (btx *BitBitx) GetDepth(size int, currency goex.CurrencyPair) (*goex.Depth, error) {
	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%v", size))
	resp := map[string]interface{}{}
	err := btx.doRequest("GET", DEPTH_URI+"/"+currency.ToSymbol("")+"?"+params.Encode(), &resp)
	if err != nil {
		return nil, err
	}

	if errObj, ok := resp["error"]; ok {
		return nil, errors.New(errObj.(map[string]string)["message"])
	}

	askList := []goex.DepthRecord{}

	for _, ee := range resp["ask"].([]interface{}) {
		e := ee.(map[string]interface{})
		one := goex.DepthRecord{
			Price:  goex.ToFloat64(e["price"]),
			Amount: goex.ToFloat64(e["size"]),
		}
		askList = append(askList, one)
	}

	bidList := []goex.DepthRecord{}
	for _, ee := range resp["bid"].([]interface{}) {
		e := ee.(map[string]interface{})
		one := goex.DepthRecord{
			Price:  goex.ToFloat64(e["price"]),
			Amount: goex.ToFloat64(e["size"]),
		}
		bidList = append(bidList, one)
	}

	return &goex.Depth{askList, bidList}, nil
}

func (btx *BitBitx) GetKlineRecords(currency goex.CurrencyPair, period, size, since int) ([]goex.Kline, error) {
	panic("not implement")
}

func (btx *BitBitx) GetKline(currencyPair goex.CurrencyPair, period string, size, since int64) ([]goex.Kline, error) {
	switch period {
	case "M1", "M3", "M5", "M15", "M30", "H1", "H4", "D1", "D7", "1M":
	default:
		return nil, errors.New("Invalid period")
	}
	if size < 0 {
		return nil, errors.New("Invalid size")
	}

	params := url.Values{}
	params.Set("period", period)
	if size > 0 {
		params.Set("limit", fmt.Sprintf("%v", size))
	}
	resp := []map[string]interface{}{}
	err := btx.doRequest("GET", KLINE_URI+"/"+currencyPair.ToSymbol("")+"?"+params.Encode(), &resp)
	if err != nil {
		return nil, err
	}

	klines := []goex.Kline{}
	for _, e := range resp {
		one := goex.Kline{
			Timestamp: parseTime(e["timestamp"].(string)),
			Open:      goex.ToFloat64(e["open"]),
			Close:     goex.ToFloat64(e["close"]),
			High:      goex.ToFloat64(e["high"]),
			Low:       goex.ToFloat64(e["low"]),
			Vol:       goex.ToFloat64(e["volume"]), // base currency, eg: ETH for pair ETHBTC
		}
		klines = append(klines, one)
	}
	return klines, nil
}

func (btx *BitBitx) GetTrades(currencyPair goex.CurrencyPair, since int64) ([]goex.Trade, error) {
	params := url.Values{}
	timestamp := time.Unix(since, 0).Format("2006-01-02T15:04:05")
	params.Set("from", timestamp)
	resp := []map[string]interface{}{}
	err := btx.doRequest("GET", TRADES_URI+"/"+currencyPair.ToSymbol("")+"?"+params.Encode(), &resp)
	if err != nil {
		return nil, err
	}

	trades := []goex.Trade{}
	for _, e := range resp {
		one := goex.Trade{
			Tid:    int64(goex.ToUint64(e["id"])),
			Type:   e["side"].(string),
			Amount: goex.ToFloat64(e["quantity"]),
			Price:  goex.ToFloat64(e["price"]),
			Date:   parseTime(e["timestamp"].(string)),
		}
		trades = append(trades, one)
	}
	return trades, nil
}
*/

func (btx *BitBitx) doRequest(params string) ([]byte, error) {
	encData, err := goex.Des3ECBEncrypt([]byte(params), []byte(btx.des3Key))
	if err != nil {
		return nil, errors.New("DES3 encrypt error:" + err.Error())
	}

	base64Str := base64.StdEncoding.EncodeToString(encData)

	reqUrl := API_BASE_URL + API_METHOD
	bytes, err := goex.HttpPostForm3(btx.httpClient, reqUrl, base64Str, nil)
	if err != nil {
		return nil, err
	}

	// fmt.Println("plain res:", string(bytes))
	encBytes, err := base64.StdEncoding.DecodeString(string(bytes))
	res, err := goex.Des3ECBDecrypt(encBytes, []byte(btx.des3Key))
	if err != nil {
		return nil, errors.New("DES3 decrypt error:" + err.Error())
	}

	// fmt.Println("dec res:", string(res))
	return res, nil
}

func toOrder(resp map[string]interface{}) *goex.Order {
	return &goex.Order{
		Price:      goex.ToFloat64(resp["price"]),
		Amount:     goex.ToFloat64(resp["quantity"]),
		DealAmount: goex.ToFloat64(resp["cumQuantity"]),
		OrderID2:   resp["clientOrderId"].(string),
		OrderID:    goex.ToInt(resp["id"]),
		OrderTime:  int(parseTime(resp["createdAt"].(string))),
		Status:     parseStatus(resp["status"].(string)),
		Currency:   parseSymbol(resp["symbol"].(string)),
		Side:       parseSide(resp["side"].(string), resp["type"].(string)),
	}
}

func parseStatus(s string) goex.TradeStatus {
	var status goex.TradeStatus
	switch s {
	case "new", "suspended":
		status = goex.ORDER_UNFINISH
	case "partiallyFilled":
		status = goex.ORDER_PART_FINISH
	case "filled":
		status = goex.ORDER_FINISH
	case "canceled":
		status = goex.ORDER_CANCEL
	case "expired":
		// TODO
		status = goex.ORDER_REJECT
	default:
		panic(s)
	}
	return status
}

func parseTime(timeStr string) int64 {
	t, _ := time.Parse(time.RFC3339, timeStr) // UTC
	return t.Unix()
}

// ETHBTC --> ETH_BTC
func parseSymbol(str string) goex.CurrencyPair {
	for pair, symbol := range goex.GetExSymbols(EXCHANGE_NAME) {
		if symbol == str {
			return pair
		}
	}
	panic(str + " Not Found")
}

func parseSide(side, oType string) goex.TradeSide {
	if side == "buy" && oType == "limit" {
		return goex.BUY
	} else if side == "sell" && oType == "limit" {
		return goex.SELL
	} else if side == "buy" && oType == "market" {
		return goex.BUY_MARKET
	} else if side == "sell" && oType == "market" {
		return goex.SELL_MARKET
	} else {
		panic("Invalid TradeSide:" + side + "&" + oType)
	}
}
