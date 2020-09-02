package gocian

import (
	"encoding/json"
	"io/ioutil"
	"github.com/go-errors/errors"
	"strconv"
	"fmt"
	"strings"
	"time"
)

const CIAN_CONFIG_NAME = "cian_conf.json"

const (
	MOSCOW_HOST           = "cian.ru"
	SAINT_PETERSBURG_HOST = "spb.cian.ru"

	CURRENCY_RUBLE = 2

	DEAL_TYPE_SALE = "sale" // покупка
	DEAL_TYPE_RENT = "rent" // аренда

	ENGINE_VERSION_LEGACY = 1
	ENGINE_VERSION_NEW    = 2

	MIN_ROOMS = 1
	MAX_ROOMS = 6
)

type CianConf struct {
	Host          string
	CurrencyID    int
	DealType      string
	EngineVersion int

	MinPrice int
	MaxPrice int

	MinRoomsAmount int
	MaxRoomsAmount int

	MinArea int
	MaxArea int

	MinLivingArea int
	MaxLivingArea int

	MinCeilingHeight float32

	AllowFirstFloor  bool
	AllowSecondFloor bool

	IpotekaIsPossible bool

	AdditionalParams string

	OfferKeepInMemoryPeriod time.Duration
}

func (cc *CianConf) err(fieldName string) error {
	return errors.New("Unknown " + fieldName + " in cian configuration")
}

func (cc *CianConf) GetUrl(page int) (*string, error) {
	if cc.Host != MOSCOW_HOST && cc.Host != SAINT_PETERSBURG_HOST {
		return nil, cc.err("host")
	}
	if cc.CurrencyID != CURRENCY_RUBLE {
		return nil, cc.err("currency")
	}
	if cc.DealType != DEAL_TYPE_SALE && cc.DealType != DEAL_TYPE_RENT {
		return nil, cc.err("deal type")
	}
	if cc.EngineVersion != ENGINE_VERSION_LEGACY && cc.EngineVersion != ENGINE_VERSION_NEW {
		return nil, cc.err("engine version")
	}
	if page < 1 {
		return nil, cc.err("page number")
	}
	url := "https://" + cc.Host + "/cat.php?"
	params := map[string]interface{}{
		"currency":       cc.CurrencyID,
		"deal_type":      cc.DealType,
		"engine_version": cc.EngineVersion,
		"offer_type":     "flat",
		"sort":           "price_object_order",
	}
	if page > 1 {
		params["p"] = page
	}
	if cc.MinPrice != 0 {
		params["minprice"] = cc.MinPrice
	}
	if cc.MaxPrice != 0 {
		params["maxprice"] = cc.MaxPrice
	}
	if cc.MinRoomsAmount != 0 && cc.MaxRoomsAmount == 0 {
		cc.MaxRoomsAmount = MAX_ROOMS
	} else if cc.MinRoomsAmount == 0 && cc.MaxRoomsAmount != 0 {
		cc.MinRoomsAmount = MIN_ROOMS
	}
	if cc.MaxRoomsAmount < cc.MinRoomsAmount {
		return nil, cc.err("rooms amount")
	}
	if cc.MinRoomsAmount != 0 && cc.MaxRoomsAmount != 0 {
		for i := cc.MinRoomsAmount; i <= cc.MaxRoomsAmount; i++ {
			params["room"+strconv.Itoa(i)] = 1
		}
	}
	if cc.MinArea != 0 {
		params["mintarea"] = cc.MinArea
	}
	if cc.MaxArea != 0 {
		params["maxtarea"] = cc.MaxArea
	}
	if cc.MinLivingArea != 0 {
		params["minlarea"] = cc.MinLivingArea
	}
	if cc.MaxLivingArea != 0 {
		params["maxlarea"] = cc.MaxLivingArea
	}
	if cc.MinCeilingHeight != 0 {
		params["min_ceiling_height"] = cc.MinCeilingHeight
	}
	if !cc.AllowFirstFloor {
		params["is_first_floor"] = 0;
	}
	if cc.IpotekaIsPossible {
		params["ipoteka"] = 1
	}
	var strParams []string
	for key, value := range params {
		strParams = append(strParams, fmt.Sprintf("%s=%v", key, value))
	}
	url = url + strings.Join(strParams, "&")
	if cc.AdditionalParams != "" {
		url = url + "&" + cc.AdditionalParams
	}
	return &url, nil
}

func ReadCianConf() (*CianConf, error) {
	conf := &CianConf{
		Host:                    SAINT_PETERSBURG_HOST,
		CurrencyID:              CURRENCY_RUBLE,
		EngineVersion:           ENGINE_VERSION_NEW,
		AdditionalParams:        "",
		OfferKeepInMemoryPeriod: time.Hour * 24 * 7,
	}
	file, err := ioutil.ReadFile(CIAN_CONFIG_NAME)
	if err == nil {
		if err = json.Unmarshal([]byte(file), conf); err == nil {
			return conf, nil
		}
	}
	conf.DealType = DEAL_TYPE_SALE
	data, err := json.MarshalIndent(*conf, "", "\n")
	if err != nil {
		return nil, err
	}
	if err = ioutil.WriteFile(CIAN_CONFIG_NAME, data, 0644); err != nil {
		return nil, err
	}
	return conf, nil
}
