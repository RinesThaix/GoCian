package gocian

import (
	"net/http"
	"io/ioutil"
	"github.com/go-errors/errors"
	"strconv"
	"io"
	"strings"

	"github.com/json-iterator/go"
	"os"
	"time"
	"log"
)

const (
	JSON_DATA_LINE_PREFIX = "window._cianConfig['frontend-serp'] = "
	CAPTCHA               = "<div id=\"captcha\"></div>"
	CACHE_FOLDER_NAME     = "cache"
)

type CianParser struct {
	Config *CianConf
}

func (parser *CianParser) sendRequest(page int, handler func(io.ReadCloser) (interface{}, error)) (interface{}, error) {
	url, err := parser.Config.GetUrl(page)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	req, err := http.NewRequest("GET", *url, nil)
	if err != nil {
		return nil, err
	}
	// got it from google chrome to pass through captcha
	//req.AddCookie(&http.Cookie{
	//	Name:     "anti_bot",
	//	Value:    "2|1:0|10:1599057753|8:anti_bot|40:eyJyZW1vdGVfaXAiOiAiMTk0LjEwNS4yMTIuMyJ9|a5fd3128dfed4d860b07f8b40e435e8ce5b1239f582af0a91cafdf2eefcb3b57",
	//	Domain:   ".cian.ru",
	//	Expires:  time.Date(2020, time.September, 2, 17, 57, 35, 0, time.Local),
	//	Path:     "/",
	//	SameSite: http.SameSiteStrictMode,
	//	Secure:   true,
	//})
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.83 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("Status code error: " + strconv.Itoa(resp.StatusCode) + " " + resp.Status)
	}
	return handler(resp.Body)
}

func (parser *CianParser) SendRequestAndGetBody(page int) (*string, error) {
	result, err := parser.sendRequest(page, func(body io.ReadCloser) (interface{}, error) {
		bytes, err := ioutil.ReadAll(body)
		if err != nil {
			return nil, err
		}
		result := string(bytes)
		return &result, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*string), nil
}

func (parser *CianParser) checkRange(min, value, max int) bool {
	return (min == 0 || min <= value) && (max == 0 || max >= value)
}

func (parser *CianParser) parse(json jsoniter.Any) ([]CianOffer, error) {
	var offers []CianOffer
	cfg := parser.Config
	for i := 0; i < json.Size(); i++ {
		jsonOffer := json.Get(i)
		if err := jsonOffer.LastError(); err != nil {
			return nil, err
		}
		floorNumber := jsonOffer.Get("floorNumber").ToInt()
		offer := CianOffer{
			Config:      parser.Config,
			CianID:      jsonOffer.Get("cianId").ToInt(),
			Rooms:       jsonOffer.Get("roomsCount").ToInt(),
			Description: jsonOffer.Get("description").ToString(),
			TotalArea:   jsonOffer.Get("totalArea").ToFloat32(),
			LivingArea:  jsonOffer.Get("livingArea").ToFloat32(),
			FloorInfo:   jsonOffer.Get("floorNumber").ToString() + "/" + jsonOffer.Get("building", "floorsCount").ToString(),
		}
		offer.parseBargainTerms(jsonOffer.Get("bargainTerms"))
		if !parser.checkRange(cfg.MinPrice, offer.Price, cfg.MaxPrice) ||
			!parser.checkRange(cfg.MinRoomsAmount, offer.Rooms, cfg.MaxRoomsAmount) ||
			!parser.checkRange(cfg.MinArea, int(offer.TotalArea), cfg.MaxArea) ||
			!parser.checkRange(cfg.MinLivingArea, int(offer.LivingArea), cfg.MaxLivingArea) ||
			!cfg.AllowFirstFloor && floorNumber == 1 || !cfg.AllowSecondFloor && floorNumber == 2 {
			continue
		}
		offer.parseAddress(jsonOffer.Get("geo", "address"))
		offer.parsePhone(jsonOffer.Get("phones"))
		offer.parsePhotos(jsonOffer.Get("photos"))
		offers = append(offers, offer)
	}
	return offers, nil
}

func (parser *CianParser) Parse() (map[int]CianOffer, error) {
	offers := make(map[int]CianOffer)
	for page := 1; ; page++ {
		body, err := parser.SendRequestAndGetBody(page)
		if err != nil {
			return nil, err
		}
		if strings.Contains(*body, CAPTCHA) {
			return nil, errors.New("Captcha pass required")
		}
		lenBefore := len(offers)
		for _, line := range strings.Split(*body, "\n") {
			if strings.HasPrefix(line, JSON_DATA_LINE_PREFIX) {
				line = strings.ReplaceAll(line, JSON_DATA_LINE_PREFIX, "")
				bytes := []byte(line)
				parsed := jsoniter.Get(bytes, '*')
				if err = parsed.LastError(); err != nil {
					return nil, err
				}
				for i := 0; i < parsed.Size(); i++ {
					el := parsed.Get(i)
					el = el.Get("value", "results", "offers")
					if el.LastError() != nil {
						continue
					}
					parsedOffers, err := parser.parse(el)
					if err != nil {
						return nil, err
					}
					for _, offer := range parsedOffers {
						offers[offer.CianID] = offer
					}
				}
				break
			}
		}
		if lenBefore == len(offers) {
			log.Printf("parsed page %d, it was the last one", page)
			filteredOffers, err := parser.applyCaching(&offers)
			if err != nil {
				return nil, err
			}
			return *filteredOffers, nil
		} else {
			log.Printf("parsed page %d, %d new offers from here", page, len(offers)-lenBefore)
		}
	}
}

func (parser *CianParser) applyCaching(offers *map[int]CianOffer) (*map[int]CianOffer, error) {
	if parser.Config.OfferKeepInMemoryPeriod == 0 {
		return offers, nil
	}
	now := time.Now()
	if _, err := os.Stat(CACHE_FOLDER_NAME); os.IsNotExist(err) {
		if err = os.Mkdir(CACHE_FOLDER_NAME, 0777); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		files, err := ioutil.ReadDir(CACHE_FOLDER_NAME)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if file.ModTime().Add(parser.Config.OfferKeepInMemoryPeriod).Before(now) {
				if err = os.Remove(CACHE_FOLDER_NAME + "/" + file.Name()); err != nil {
					return nil, err
				}
			}
		}
	}
	result := make(map[int]CianOffer)
	for cianID, offer := range *offers {
		fileName := CACHE_FOLDER_NAME + "/" + strconv.Itoa(cianID)
		if _, err := os.Stat(fileName); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if _, err := os.Create(fileName); err != nil {
			return nil, err
		}
		result[cianID] = offer
	}
	return &result, nil
}
