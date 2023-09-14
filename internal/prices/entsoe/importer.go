package entsoe

import (
	"context"
	"encoding/xml"
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/domain/arithmetic"
	"enman/internal/log"
	"enman/internal/prices"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
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

func (r resolution) toDuration() (time.Duration, error) {
	switch r {
	case p1y:
		return 0, fmt.Errorf("duration of year not supported")
	case p1m:
		return 0, fmt.Errorf("duration of month not supported")
	case p7d:
		return 0, fmt.Errorf("duration of week not supported")
	case pt60m:
		return time.Hour, nil
	case pt30m:
		return time.Minute * 30, nil
	case pt15m:
		return time.Minute * 15, nil
	default:
		return 0, fmt.Errorf("invalid resulution %v", r)
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

type PriceImporter struct {
	prices.PriceImporter
	domain           string
	securityToken    string
	energyProviders  []config.EnergyProvider
	repository       domain.Repository
	registeredEvents map[string]bool
	eventMutex       sync.Mutex
}

func (e *PriceImporter) ImportPrices(ctx context.Context, startDate time.Time, endDate time.Time) error {
	if log.InfoEnabled() {
		log.Info("Start reading energy prices from ENTSO-E")
	}
	if !startDate.Before(endDate) {
		return fmt.Errorf("start date must be before end date")
	}
	url := fmt.Sprintf("https://web-api.tp.entsoe.eu/api?securityToken=%s&documentType=A44&in_Domain=%s&out_Domain=%s&periodStart=%s&periodEnd=%s", e.securityToken, e.domain, e.domain, startDate.Truncate(time.Hour).UTC().Format("200601021504"), endDate.Truncate(time.Hour).UTC().Format("200601021504"))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(req)
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
		interval, _ := period.Resolution.toDuration()
		end := period.Resolution.add(start)
		for j := 0; j < len(doc.TimeSeries[i].Period.Point); j++ {
			point := doc.TimeSeries[i].Period.Point[j]
			price := point.PriceAmount * units.toKwhFactor()
			entsoePrice := &domain.EnergyPrice{
				Time:             start,
				ConsumptionPrice: price,
				FeedbackPrice:    price,
				Provider:         "ENTSO-E",
			}
			e.repository.StoreEnergyPrice(entsoePrice)
			go e.firePriceChangedEvent(ctx, entsoePrice, interval)
			for k := 0; k < len(e.energyProviders); k++ {
				energyPrice := e.calculateProviderPrice(e.energyProviders[k], price, start)
				if energyPrice != nil {
					e.repository.StoreEnergyPrice(energyPrice)
					go e.firePriceChangedEvent(ctx, energyPrice, interval)
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

func (e *PriceImporter) firePriceChangedEvent(ctx context.Context, price *domain.EnergyPrice, interval time.Duration) {
	event := domain.NewElectricityPriceValues().
		SetConsumptionPrice(price.ConsumptionPrice).
		SetFeedbackPrice(price.FeedbackPrice).
		SetEnergyProviderName(price.Provider).
		SetPriceStartingTime(price.Time)
	if interval != 0 {
		event.SetInterval(interval)
	}
	eventKey := fmt.Sprintf("%v-%v", event.EnergyProviderName(), event.PriceStartingTime())
	if time.Now().After(price.Time) {
		if log.DebugEnabled() {
			log.Debugf("Not registering price task because it was in the past %s", eventKey)
		}
		return
	}
	e.eventMutex.Lock()
	_, ok := e.registeredEvents[eventKey]
	if ok {
		if log.DebugEnabled() {
			log.Debugf("Not registering price task because it was already registered %s", eventKey)
		}
		e.eventMutex.Unlock()
		return
	}
	if log.DebugEnabled() {
		log.Debugf("Registering price task %s", eventKey)
	}
	e.registeredEvents[eventKey] = true
	e.eventMutex.Unlock()
	timer := time.NewTimer(time.Until(price.Time))
	defer timer.Stop()

	select {
	case <-timer.C:
		domain.ElectricityPrices.Trigger(event)
		e.eventMutex.Lock()
		delete(e.registeredEvents, eventKey)
		e.eventMutex.Unlock()
		if log.DebugEnabled() {
			log.Debugf("Deregistered price task because it was fired %s", eventKey)
		}
		return
	case <-ctx.Done():
		return
	}
}

func (e *PriceImporter) calculateProviderPrice(provider config.EnergyProvider, price float32, time time.Time) *domain.EnergyPrice {
	p := &domain.EnergyPrice{
		Provider: provider.Name,
		Time:     time,
	}
	sort.Slice(provider.PriceModels, func(i, j int) bool {
		return provider.PriceModels[i].StartAsTime().Before(provider.PriceModels[j].StartAsTime())
	})
	ix := math.MinInt
	for i := 0; i < len(provider.PriceModels); i++ {
		if provider.PriceModels[i].StartAsTime().After(time) {
			break
		}
		ix = i
	}
	if ix < 0 {
		return nil
	}
	// consumption price
	formula := provider.PriceModels[ix].ConsumptionFormula
	if formula != "" {
		value, err := arithmetic.ParseExpression(formula, map[string]float64{"entso-e": float64(price)})
		if err != nil {
			log.Warningf("Unable to calculate consumption price: %v", err)
		} else {
			p.ConsumptionPrice = float32(value)
		}
	}
	// feedback price
	formula = provider.PriceModels[ix].FeedbackFormula
	if formula != "" {
		value, err := arithmetic.ParseExpression(formula, map[string]float64{"entso-e": float64(price)})
		if err != nil {
			log.Warningf("Unable to calculate feedback price: %v", err)
		} else {
			p.FeedbackPrice = float32(value)
		}
	}
	return p
}

func NewEntsoeImporter(county string, area string, securityToken string, energyProviders []config.EnergyProvider, repository domain.Repository) (*PriceImporter, error) {
	entsoeDomain := ""
	key := strings.ToUpper(county)
	if area != "" {
		key += "_" + strings.ToUpper(area)
	}
	// See https://transparency.entsoe.eu/content/static_content/Static%20content/web%20api/Guide.html#_areas
	switch key {
	case "AL":
		entsoeDomain = "10YAL-KESH-----5"
	case "AT":
		entsoeDomain = "10YAT-APG------L"
	case "BA":
		entsoeDomain = "10YBA-JPCC-----D"
	case "BE":
		entsoeDomain = "10YBE----------2"
	case "BG":
		entsoeDomain = "10YCA-BULGARIA-R"
	case "BY":
		entsoeDomain = "10Y1001A1001A51S"
	case "CH":
		entsoeDomain = "10YCH-SWISSGRIDZ"
	case "CWE":
		entsoeDomain = "10YDOM-REGION-1V"
	case "CY":
		entsoeDomain = "10YCY-1001A0003J"
	case "CZ":
		entsoeDomain = "10YCZ-CEPS-----N"
	case "CZ_DE_SK":
		entsoeDomain = "10YDOM-CZ-DE-SKK"
	case "DE":
		entsoeDomain = "10Y1001A1001A83F"
	case "DE_50HZ":
		entsoeDomain = "10YDE-VE-------2"
	case "DE_AMPRION":
		entsoeDomain = "10YDE-RWENET---I"
	case "DE_AT_LU":
		entsoeDomain = "10Y1001A1001A63L"
	case "DE_LU":
		entsoeDomain = "10Y1001A1001A82H"
	case "DE_TENNET":
		entsoeDomain = "10YDE-EON------1"
	case "DE_TRANSNET":
		entsoeDomain = "10YDE-ENBW-----N"
	case "DK":
		entsoeDomain = "10Y1001A1001A65H"
	case "DK_1":
		entsoeDomain = "10YDK-1--------W"
	case "DK_1_NO_1":
		entsoeDomain = "46Y000000000007M"
	case "DK_2":
		entsoeDomain = "10YDK-2--------M"
	case "DK_CA":
		entsoeDomain = "10Y1001A1001A796"
	case "EE":
		entsoeDomain = "10Y1001A1001A39I"
	case "ES":
		entsoeDomain = "10YES-REE------0"
	case "FI":
		entsoeDomain = "10YFI-1--------U"
	case "FR":
		entsoeDomain = "10YFR-RTE------C"
	case "GB":
		entsoeDomain = "10YGB----------A"
	case "GB_ELECLINK":
		entsoeDomain = "11Y0-0000-0265-K"
	case "GB_IFA":
		entsoeDomain = "10Y1001C--00098F"
	case "GB_IFA2":
		entsoeDomain = "17Y0000009369493"
	case "GB_NIR":
		entsoeDomain = "10Y1001A1001A016"
	case "GR":
		entsoeDomain = "10YGR-HTSO-----Y"
	case "HR":
		entsoeDomain = "10YHR-HEP------M"
	case "HU":
		entsoeDomain = "10YHU-MAVIR----U"
	case "IE":
		entsoeDomain = "10YIE-1001A00010"
	case "IE_SEM":
		entsoeDomain = "10Y1001A1001A59C"
	case "IS":
		entsoeDomain = "IS"
	case "IT":
		entsoeDomain = "10YIT-GRTN-----B"
	case "IT_BRNN":
		entsoeDomain = "10Y1001A1001A699"
	case "IT_CALA":
		entsoeDomain = "10Y1001C--00096J"
	case "IT_CNOR":
		entsoeDomain = "10Y1001A1001A70O"
	case "IT_CSUD":
		entsoeDomain = "10Y1001A1001A71M"
	case "IT_FOGN":
		entsoeDomain = "10Y1001A1001A72K"
	case "IT_GR":
		entsoeDomain = "10Y1001A1001A66F"
	case "IT_MACRO_NORTH":
		entsoeDomain = "10Y1001A1001A84D"
	case "IT_MACRO_SOUTH":
		entsoeDomain = "10Y1001A1001A85B"
	case "IT_MALTA":
		entsoeDomain = "10Y1001A1001A877"
	case "IT_NORD":
		entsoeDomain = "10Y1001A1001A73I"
	case "IT_NORD_AT":
		entsoeDomain = "10Y1001A1001A80L"
	case "IT_NORD_CH":
		entsoeDomain = "10Y1001A1001A68B"
	case "IT_NORD_FR":
		entsoeDomain = "10Y1001A1001A81J"
	case "IT_NORD_SI":
		entsoeDomain = "10Y1001A1001A67D"
	case "IT_PRGP":
		entsoeDomain = "10Y1001A1001A76C"
	case "IT_ROSN":
		entsoeDomain = "10Y1001A1001A77A"
	case "IT_SACO_AC":
		entsoeDomain = "10Y1001A1001A885"
	case "IT_SACO_DC":
		entsoeDomain = "10Y1001A1001A893"
	case "IT_SARD":
		entsoeDomain = "10Y1001A1001A74G"
	case "IT_SICI":
		entsoeDomain = "10Y1001A1001A75E"
	case "IT_SUD":
		entsoeDomain = "10Y1001A1001A788"
	case "LV":
		entsoeDomain = "10YLV-1001A00074"
	case "LT":
		entsoeDomain = "10YLT-1001A0008Q"
	case "LU":
		entsoeDomain = "10YLU-CEGEDEL-NQ"
	case "MD":
		entsoeDomain = "10Y1001A1001A990"
	case "ME":
		entsoeDomain = "10YCS-CG-TSO---S"
	case "MK":
		entsoeDomain = "10YMK-MEPSO----8"
	case "MT":
		entsoeDomain = "10Y1001A1001A93C"
	case "NL":
		entsoeDomain = "10YNL----------L"
	case "NO":
		entsoeDomain = "10YNO-0--------C"
	case "NO_1":
		entsoeDomain = "10YNO-1--------2"
	case "NO_1A":
		entsoeDomain = "10Y1001A1001A64J"
	case "NO_2":
		entsoeDomain = "10YNO-2--------T"
	case "NO_2_NSL":
		entsoeDomain = "50Y0JVU59B4JWQCU"
	case "NO_2A":
		entsoeDomain = "10Y1001C--001219"
	case "NO_3":
		entsoeDomain = "10YNO-3--------J"
	case "NO_4":
		entsoeDomain = "10YNO-4--------9"
	case "NO_5":
		entsoeDomain = "10Y1001A1001A48H"
	case "PL":
		entsoeDomain = "10YPL-AREA-----S"
	case "PL_CZ":
		entsoeDomain = "10YDOM-1001A082L"
	case "PT":
		entsoeDomain = "10YPT-REN------W"
	case "RO":
		entsoeDomain = "10YRO-TEL------P"
	case "RS":
		entsoeDomain = "10YCS-SERBIATSOV"
	case "RU":
		entsoeDomain = "10Y1001A1001A49F"
	case "RU_KGD":
		entsoeDomain = "10Y1001A1001A50U"
	case "SE":
		entsoeDomain = "10YSE-1--------K"
	case "SE_1":
		entsoeDomain = "10Y1001A1001A44P"
	case "SE_2":
		entsoeDomain = "10Y1001A1001A45N"
	case "SE_3":
		entsoeDomain = "10Y1001A1001A46L"
	case "SE_4":
		entsoeDomain = "10Y1001A1001A47J"
	case "SI":
		entsoeDomain = "10YSI-ELES-----O"
	case "SK":
		entsoeDomain = "10YSK-SEPS-----K"
	case "TR":
		entsoeDomain = "10YTR-TEIAS----W"
	case "UA":
		entsoeDomain = "10Y1001C--00003F"
	case "UA_BEI":
		entsoeDomain = "10YUA-WEPS-----0"
	case "UA_DOBTPP":
		entsoeDomain = "10Y1001A1001A869"
	case "UA_IPS":
		entsoeDomain = "10Y1001C--000182"
	case "UK":
		entsoeDomain = "10Y1001A1001A92E"
	case "XK":
		entsoeDomain = "10Y1001C--00100H"
	default:
		return nil, fmt.Errorf("invalid country (%s) & area (%s) combination", county, area)
	}
	return &PriceImporter{
		domain:           entsoeDomain,
		securityToken:    securityToken,
		energyProviders:  energyProviders,
		repository:       repository,
		registeredEvents: make(map[string]bool),
	}, nil
}
