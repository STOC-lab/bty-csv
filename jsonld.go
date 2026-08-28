package main

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var reLD = regexp.MustCompile(`(?s)<script[^>]*application/ld\+json[^>]*>(.*?)</script>`)

// SalonLD は JSON-LD の schema.org 表現から必要な項目を取り出す。
// HairSalon / BeautySalon / NailSalon などの @type が来る。
type SalonLD struct {
	Type        string          `json:"@type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	PriceRange  string          `json:"priceRange"`
	Telephone   string          `json:"telephone"`
	Address     json.RawMessage `json:"address"`
	URL         string          `json:"url"`
	Geo         *struct {
		Lat float64 `json:"latitude"`
		Lng float64 `json:"longitude"`
	} `json:"geo"`
	AggregateRating *struct {
		RatingValue json.Number `json:"ratingValue"`
		ReviewCount json.Number `json:"reviewCount"`
	} `json:"aggregateRating"`
}

// AddressText は address が文字列でも PostalAddress オブジェクトでも読む。
func (s *SalonLD) AddressText() string {
	if len(s.Address) == 0 {
		return ""
	}
	var str string
	if json.Unmarshal(s.Address, &str) == nil {
		return str
	}
	var pa struct {
		Name          string `json:"name"`
		StreetAddress string `json:"streetAddress"`
		PostalCode    string `json:"postalCode"`
		Region        string `json:"addressRegion"`
		Locality      string `json:"addressLocality"`
	}
	if json.Unmarshal(s.Address, &pa) == nil {
		if pa.Name != "" {
			return pa.Name
		}
		return pa.Region + pa.Locality + pa.StreetAddress
	}
	return ""
}

var salonTypes = map[string]bool{
	"HairSalon": true, "BeautySalon": true, "NailSalon": true,
	"DaySpa": true, "HealthAndBeautyBusiness": true, "MedicalClinic": true,
	"LocalBusiness": true,
}

// ExtractSalonLD はページ内の全 JSON-LD からサロン本体のものを選ぶ。
// 複数ヒットする場合は、情報量の多い方（電話・評価を持つ方）を採用する。
func ExtractSalonLD(body string) *SalonLD {
	var best *SalonLD
	score := func(s *SalonLD) int {
		n := 0
		for _, v := range []string{s.Telephone, s.AddressText(), s.Description, s.PriceRange} {
			if v != "" {
				n++
			}
		}
		if s.AggregateRating != nil {
			n += 2
		}
		if s.Geo != nil {
			n++
		}
		return n
	}

	for _, m := range reLD.FindAllStringSubmatch(body, -1) {
		raw := strings.TrimSpace(m[1])
		if raw == "" {
			continue
		}
		// 単体オブジェクトと配列の両方が来る
		var one SalonLD
		if err := json.Unmarshal([]byte(raw), &one); err == nil && salonTypes[one.Type] {
			if best == nil || score(&one) > score(best) {
				c := one
				best = &c
			}
			continue
		}
		var many []SalonLD
		if err := json.Unmarshal([]byte(raw), &many); err == nil {
			for i := range many {
				if !salonTypes[many[i].Type] {
					continue
				}
				if best == nil || score(&many[i]) > score(best) {
					c := many[i]
					best = &c
				}
			}
		}
	}
	return best
}

// BreadcrumbArea はパンくずから広域・中エリア名を取り出す。
// 例: 総合トップ > ヘアサロン検索トップ > 関西トップ > 西宮・伊丹・芦屋・尼崎トップ > 店名
func BreadcrumbArea(body string) (svc, area string) {
	for _, m := range reLD.FindAllStringSubmatch(body, -1) {
		var bc struct {
			Type string `json:"@type"`
			Item []struct {
				Position int `json:"position"`
				Item     struct {
					ID   string `json:"@id"`
					Name string `json:"name"`
				} `json:"item"`
			} `json:"itemListElement"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(m[1])), &bc) != nil {
			continue
		}
		if bc.Type != "BreadcrumbList" {
			continue
		}
		for _, e := range bc.Item {
			n := strings.TrimSuffix(e.Item.Name, "トップ")
			switch e.Position {
			case 3:
				svc = n
			case 4:
				area = n
			}
		}
	}
	return
}

func ldFloat(n json.Number) string {
	if n == "" {
		return ""
	}
	if f, err := strconv.ParseFloat(string(n), 64); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return string(n)
}
