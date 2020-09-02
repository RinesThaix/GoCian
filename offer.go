package gocian

import (
	"github.com/json-iterator/go"
	"strings"
	"fmt"
)

type CianOffer struct {
	Config      *CianConf
	CianID      int
	Rooms       int
	Description string

	TotalArea  float32
	LivingArea float32

	Address   string
	FloorInfo string

	SaleType string
	Price    int

	PhotoURLs []string

	Phone string
}

func (offer *CianOffer) parseAddress(address jsoniter.Any) {
	var parts []string
	for i := 0; i < address.Size(); i++ {
		parts = append(parts, address.Get(i, "title").ToString())
	}
	offer.Address = strings.Join(parts, ", ")
}

func (offer *CianOffer) parseBargainTerms(terms jsoniter.Any) {
	offer.Price = terms.Get("price").ToInt()
	offer.SaleType = terms.Get("saleType").ToString()
}

func (offer *CianOffer) parsePhone(phones jsoniter.Any) {
	for i := 0; i < phones.Size(); i++ {
		phone := phones.Get(i)
		offer.Phone = fmt.Sprintf("+%s%s", phone.Get("countryCode").ToString(), phone.Get("number").ToString())
		break
	}
}

func (offer *CianOffer) parsePhotos(photos jsoniter.Any) {
	for i := 0; i < photos.Size(); i++ {
		photo := photos.Get(i)
		url := photo.Get("fullUrl").ToString()
		url = strings.ReplaceAll(url, "\u002F", "/")
		offer.PhotoURLs = append(offer.PhotoURLs, url)
	}
}

func (offer *CianOffer) GetSaleType() string {
	if offer.SaleType == "free" {
		return "свободная"
	} else if offer.SaleType == "alternative" {
		return "альтернативная"
	} else {
		return offer.SaleType
	}
}

func (offer *CianOffer) GetCianUrl() string {
	return fmt.Sprintf("https://%s/%s/flat/%d", offer.Config.Host, offer.Config.DealType, offer.CianID)
}

func (offer *CianOffer) ToString() string {
	description := strings.ReplaceAll(offer.Description, "\n", " ")
	res := fmt.Sprintf("%s\nАдрес: %s\n", offer.GetCianUrl(), offer.Address)
	res = res + fmt.Sprintf("Цена: %d₽\nКомнат: %d\nПлощадь: %.2f м², жилая: %.2f м²\n", offer.Price, offer.Rooms, offer.TotalArea, offer.LivingArea)
	res = res + fmt.Sprintf("Этаж: %s\nТип продажи: %s\nОписание: %s\nТелефон для связи: %s\n\n", offer.FloorInfo, offer.GetSaleType(), description, offer.Phone)
	return res
}
