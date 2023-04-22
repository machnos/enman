package entsoe

import (
	"encoding/xml"
	"enman/internal/config"
	"enman/internal/log"
	"enman/internal/persistency"
	"enman/internal/prices"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

type energyUnit string

const (
	wh  energyUnit = "WH"
	kwh            = "KWH"
	mwh            = "MWH"
	gwh            = "GWH"
)

func (eu energyUnit) toKwhFactor() float32 {
	switch eu {
	case wh:
		return 1000
	case kwh:
		return 1
	case mwh:
		return 0.001
	case gwh:
		return 0.000001
	default:
		return 1
	}
}

type resolution string

const (
	p1y   = "P1Y"
	p1m   = "P1M"
	p7d   = "P7D"
	pt60m = "PT60M"
	pt30m = "PT30M"
	pt15m = "PT15M"
)

func (r resolution) add(source time.Time) time.Time {
	switch r {
	case p1y:
		return source.AddDate(1, 0, 0)
	case p1m:
		return source.AddDate(0, 1, 0)
	case p7d:
		return source.AddDate(0, 0, 7)
	case pt60m:
		return source.Add(time.Hour)
	case pt30m:
		return source.Add(time.Minute * 30)
	case pt15m:
		return source.Add(time.Minute * 15)
	default:
		return source
	}
}

type publicationMarketDocument struct {
	XMLName                     xml.Name `xml:"Publication_MarketDocument"`
	Text                        string   `xml:",chardata"`
	Xmlns                       string   `xml:"xmlns,attr"`
	MRID                        string   `xml:"mRID"`
	RevisionNumber              string   `xml:"revisionNumber"`
	Type                        string   `xml:"type"`
	SenderMarketParticipantMRID struct {
		Text         string `xml:",chardata"`
		CodingScheme string `xml:"codingScheme,attr"`
	} `xml:"sender_MarketParticipant.mRID"`
	SenderMarketParticipantMarketRoleType string `xml:"sender_MarketParticipant.marketRole.type"`
	ReceiverMarketParticipantMRID         struct {
		Text         string `xml:",chardata"`
		CodingScheme string `xml:"codingScheme,attr"`
	} `xml:"receiver_MarketParticipant.mRID"`
	ReceiverMarketParticipantMarketRoleType string `xml:"receiver_MarketParticipant.marketRole.type"`
	CreatedDateTime                         string `xml:"createdDateTime"`
	PeriodTimeInterval                      struct {
		Text  string `xml:",chardata"`
		Start string `xml:"start"`
		End   string `xml:"end"`
	} `xml:"period.timeInterval"`
	TimeSeries []struct {
		Text         string `xml:",chardata"`
		MRID         string `xml:"mRID"`
		BusinessType string `xml:"businessType"`
		InDomainMRID struct {
			Text         string `xml:",chardata"`
			CodingScheme string `xml:"codingScheme,attr"`
		} `xml:"in_Domain.mRID"`
		OutDomainMRID struct {
			Text         string `xml:",chardata"`
			CodingScheme string `xml:"codingScheme,attr"`
		} `xml:"out_Domain.mRID"`
		CurrencyUnitName     string     `xml:"currency_Unit.name"`
		PriceMeasureUnitName energyUnit `xml:"price_Measure_Unit.name"`
		CurveType            string     `xml:"curveType"`
		Period               struct {
			Text         string `xml:",chardata"`
			TimeInterval struct {
				Text  string `xml:",chardata"`
				Start string `xml:"start"`
				End   string `xml:"end"`
			} `xml:"timeInterval"`
			Resolution resolution `xml:"resolution"`
			Point      []struct {
				Text        string  `xml:",chardata"`
				Position    uint16  `xml:"position"`
				PriceAmount float32 `xml:"price.amount"`
				Quantity    string  `xml:"quantity"`
			} `xml:"Point"`
		} `xml:"Period"`
		AuctionType                                              string `xml:"auction.type"`
		ContractMarketAgreementType                              string `xml:"contract_MarketAgreement.type"`
		QuantityMeasureUnitName                                  string `xml:"quantity_Measure_Unit.name"`
		AuctionMRID                                              string `xml:"auction.mRID"`
		AuctionCategory                                          string `xml:"auction.category"`
		ClassificationSequenceAttributeInstanceComponentPosition string `xml:"classificationSequence_AttributeInstanceComponent.position"`
	} `xml:"TimeSeries"`
}

type EntsoePriceImporter struct {
	prices.PriceImporter
	domain          string
	securityToken   string
	energyProviders []config.EnergyProvider
	repository      persistency.Repository
}

func (e *EntsoePriceImporter) ImportPrices(startDate time.Time, endDate time.Time) error {
	if log.InfoEnabled() {
		log.Info("Start reading energy prices from ENTSO-E")
	}
	if !startDate.Before(endDate) {
		return fmt.Errorf("start date must be before end date")
	}
	url := fmt.Sprintf("https://web-api.tp.entsoe.eu/api?securityToken=%s&documentType=A44&in_Domain=%s&out_Domain=%s&periodStart=%s&periodEnd=%s", e.securityToken, e.domain, e.domain, startDate.Truncate(time.Hour).UTC().Format("200601021504"), endDate.Truncate(time.Hour).UTC().Format("200601021504"))
	response, err := http.DefaultClient.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)
	body, _ := io.ReadAll(response.Body)
	var doc publicationMarketDocument
	err = xml.Unmarshal(body, &doc)
	if err != nil {
		return err
	}
	for i := 0; i < len(doc.TimeSeries); i++ {
		units := doc.TimeSeries[i].PriceMeasureUnitName
		period := doc.TimeSeries[i].Period
		start, _ := time.Parse("2006-01-02T15:04Z07", period.TimeInterval.Start)
		end := period.Resolution.add(start)
		for j := 0; j < len(doc.TimeSeries[i].Period.Point); j++ {
			point := doc.TimeSeries[i].Period.Point[j]
			price := point.PriceAmount * units.toKwhFactor()
			e.repository.StoreEnergyPrice(&prices.EnergyPrice{
				Time:     start,
				Price:    price,
				Provider: "ENTSO-E",
			})
			for k := 0; k < len(e.energyProviders); k++ {
				energyPrice := e.calculateProviderPrice(e.energyProviders[k], price, start)
				if energyPrice != nil {
					e.repository.StoreEnergyPrice(energyPrice)
				}
			}
			start = end
			end = period.Resolution.add(start)
		}
	}
	if log.InfoEnabled() {
		log.Info("Finished reading energy prices from ENTSO-E")
	}
	return nil
}

func (e *EntsoePriceImporter) calculateProviderPrice(provider config.EnergyProvider, price float32, time time.Time) *prices.EnergyPrice {
	p := &prices.EnergyPrice{
		Provider: provider.Name,
		Time:     time,
	}
	sort.Slice(provider.PriceModels, func(i, j int) bool {
		return provider.PriceModels[i].Start.Time().Before(provider.PriceModels[j].Start.Time())
	})
	ix := math.MinInt
	for i := 0; i < len(provider.PriceModels); i++ {
		if provider.PriceModels[i].Start.Time().After(time) {
			break
		}
		ix = i
	}
	if ix < 0 {
		return nil
	}
	p.Price = (price + provider.PriceModels[ix].AdditionalCostPerKwh) * provider.PriceModels[ix].Vat
	return p
}

func NewEntsoeImporter(county string, area string, securityToken string, energyProviders []config.EnergyProvider, repository persistency.Repository) (*EntsoePriceImporter, error) {
	domain := ""
	key := strings.ToUpper(county)
	if area != "" {
		key += "_" + strings.ToUpper(area)
	}
	// See https://transparency.entsoe.eu/content/static_content/Static%20content/web%20api/Guide.html#_areas
	switch key {
	case "AL":
		domain = "10YAL-KESH-----5"
	case "AT":
		domain = "10YAT-APG------L"
	case "BA":
		domain = "10YBA-JPCC-----D"
	case "BE":
		domain = "10YBE----------2"
	case "BG":
		domain = "10YCA-BULGARIA-R"
	case "BY":
		domain = "10Y1001A1001A51S"
	case "CH":
		domain = "10YCH-SWISSGRIDZ"
	case "CWE":
		domain = "10YDOM-REGION-1V"
	case "CY":
		domain = "10YCY-1001A0003J"
	case "CZ":
		domain = "10YCZ-CEPS-----N"
	case "CZ_DE_SK":
		domain = "10YDOM-CZ-DE-SKK"
	case "DE":
		domain = "10Y1001A1001A83F"
	case "DE_50HZ":
		domain = "10YDE-VE-------2"
	case "DE_AMPRION":
		domain = "10YDE-RWENET---I"
	case "DE_AT_LU":
		domain = "10Y1001A1001A63L"
	case "DE_LU":
		domain = "10Y1001A1001A82H"
	case "DE_TENNET":
		domain = "10YDE-EON------1"
	case "DE_TRANSNET":
		domain = "10YDE-ENBW-----N"
	case "DK":
		domain = "10Y1001A1001A65H"
	case "DK_1":
		domain = "10YDK-1--------W"
	case "DK_1_NO_1":
		domain = "46Y000000000007M"
	case "DK_2":
		domain = "10YDK-2--------M"
	case "DK_CA":
		domain = "10Y1001A1001A796"
	case "EE":
		domain = "10Y1001A1001A39I"
	case "ES":
		domain = "10YES-REE------0"
	case "FI":
		domain = "10YFI-1--------U"
	case "FR":
		domain = "10YFR-RTE------C"
	case "GB":
		domain = "10YGB----------A"
	case "GB_ELECLINK":
		domain = "11Y0-0000-0265-K"
	case "GB_IFA":
		domain = "10Y1001C--00098F"
	case "GB_IFA2":
		domain = "17Y0000009369493"
	case "GB_NIR":
		domain = "10Y1001A1001A016"
	case "GR":
		domain = "10YGR-HTSO-----Y"
	case "HR":
		domain = "10YHR-HEP------M"
	case "HU":
		domain = "10YHU-MAVIR----U"
	case "IE":
		domain = "10YIE-1001A00010"
	case "IE_SEM":
		domain = "10Y1001A1001A59C"
	case "IS":
		domain = "IS"
	case "IT":
		domain = "10YIT-GRTN-----B"
	case "IT_BRNN":
		domain = "10Y1001A1001A699"
	case "IT_CALA":
		domain = "10Y1001C--00096J"
	case "IT_CNOR":
		domain = "10Y1001A1001A70O"
	case "IT_CSUD":
		domain = "10Y1001A1001A71M"
	case "IT_FOGN":
		domain = "10Y1001A1001A72K"
	case "IT_GR":
		domain = "10Y1001A1001A66F"
	case "IT_MACRO_NORTH":
		domain = "10Y1001A1001A84D"
	case "IT_MACRO_SOUTH":
		domain = "10Y1001A1001A85B"
	case "IT_MALTA":
		domain = "10Y1001A1001A877"
	case "IT_NORD":
		domain = "10Y1001A1001A73I"
	case "IT_NORD_AT":
		domain = "10Y1001A1001A80L"
	case "IT_NORD_CH":
		domain = "10Y1001A1001A68B"
	case "IT_NORD_FR":
		domain = "10Y1001A1001A81J"
	case "IT_NORD_SI":
		domain = "10Y1001A1001A67D"
	case "IT_PRGP":
		domain = "10Y1001A1001A76C"
	case "IT_ROSN":
		domain = "10Y1001A1001A77A"
	case "IT_SACO_AC":
		domain = "10Y1001A1001A885"
	case "IT_SACO_DC":
		domain = "10Y1001A1001A893"
	case "IT_SARD":
		domain = "10Y1001A1001A74G"
	case "IT_SICI":
		domain = "10Y1001A1001A75E"
	case "IT_SUD":
		domain = "10Y1001A1001A788"
	case "LV":
		domain = "10YLV-1001A00074"
	case "LT":
		domain = "10YLT-1001A0008Q"
	case "LU":
		domain = "10YLU-CEGEDEL-NQ"
	case "MD":
		domain = "10Y1001A1001A990"
	case "ME":
		domain = "10YCS-CG-TSO---S"
	case "MK":
		domain = "10YMK-MEPSO----8"
	case "MT":
		domain = "10Y1001A1001A93C"
	case "NL":
		domain = "10YNL----------L"
	case "NO":
		domain = "10YNO-0--------C"
	case "NO_1":
		domain = "10YNO-1--------2"
	case "NO_1A":
		domain = "10Y1001A1001A64J"
	case "NO_2":
		domain = "10YNO-2--------T"
	case "NO_2_NSL":
		domain = "50Y0JVU59B4JWQCU"
	case "NO_2A":
		domain = "10Y1001C--001219"
	case "NO_3":
		domain = "10YNO-3--------J"
	case "NO_4":
		domain = "10YNO-4--------9"
	case "NO_5":
		domain = "10Y1001A1001A48H"
	case "PL":
		domain = "10YPL-AREA-----S"
	case "PL_CZ":
		domain = "10YDOM-1001A082L"
	case "PT":
		domain = "10YPT-REN------W"
	case "RO":
		domain = "10YRO-TEL------P"
	case "RS":
		domain = "10YCS-SERBIATSOV"
	case "RU":
		domain = "10Y1001A1001A49F"
	case "RU_KGD":
		domain = "10Y1001A1001A50U"
	case "SE":
		domain = "10YSE-1--------K"
	case "SE_1":
		domain = "10Y1001A1001A44P"
	case "SE_2":
		domain = "10Y1001A1001A45N"
	case "SE_3":
		domain = "10Y1001A1001A46L"
	case "SE_4":
		domain = "10Y1001A1001A47J"
	case "SI":
		domain = "10YSI-ELES-----O"
	case "SK":
		domain = "10YSK-SEPS-----K"
	case "TR":
		domain = "10YTR-TEIAS----W"
	case "UA":
		domain = "10Y1001C--00003F"
	case "UA_BEI":
		domain = "10YUA-WEPS-----0"
	case "UA_DOBTPP":
		domain = "10Y1001A1001A869"
	case "UA_IPS":
		domain = "10Y1001C--000182"
	case "UK":
		domain = "10Y1001A1001A92E"
	case "XK":
		domain = "10Y1001C--00100H"
	default:
		return nil, fmt.Errorf("invalid country (%s) & area (%s) combination", county, area)
	}
	return &EntsoePriceImporter{
		domain:          domain,
		securityToken:   securityToken,
		energyProviders: energyProviders,
		repository:      repository,
	}, nil
}
