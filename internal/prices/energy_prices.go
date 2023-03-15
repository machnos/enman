package prices

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

//https://api.energyzero.nl/v1/energyprices?fromDate=2023-01-19T23:00:00.000Z&tillDate=2023-01-20T22:59:59.999Z&interval=4&usageType=1&inclBtw=true

type PriceImporter interface {
	ImportPrices(startDate time.Time, endDate time.Time) error
}

type EnergyZeroPriceImporter struct {
	PriceImporter
}

func (e *EnergyZeroPriceImporter) ImportPrices(startDate time.Time, endDate time.Time) error {
	if !startDate.Before(endDate) {
		return fmt.Errorf("start date must be before end date")
	}
	url := fmt.Sprintf("https://api.energyzero.nl/v1/energyprices?fromDate=%s&tillDate=%s&interval=4&usageType=1&inclBtw=true", startDate.UTC().Format(time.RFC3339), endDate.UTC().Format(time.RFC3339))
	response, err := http.DefaultClient.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)
	body, _ := io.ReadAll(response.Body)
	println(string(body))
	return nil
}

type EasyEnergyPriceImporter struct {
	PriceImporter
}

func (e *EasyEnergyPriceImporter) ImportPrices(startDate time.Time, endDate time.Time) error {
	if !startDate.Before(endDate) {
		return fmt.Errorf("start date must be before end date")
	}
	url := fmt.Sprintf("https://mijn.easyenergy.com/nl/api/tariff/getapxtariffs?startTimestamp=%s&endTimestamp=%s&includeVat=true", startDate.UTC().Format(time.RFC3339), endDate.UTC().Format(time.RFC3339))
	println(url)
	response, err := http.DefaultClient.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)
	body, _ := io.ReadAll(response.Body)
	println(string(body))
	return nil
}

type EntsoePriceImporter struct {
	PriceImporter
	domain        string
	securityToken string
}

func (e *EntsoePriceImporter) ImportPrices(startDate time.Time, endDate time.Time) error {
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
	println(string(body))
	return nil
}

func NewEntsoeImporter(county string, area string, securityToken string) (*EntsoePriceImporter, error) {
	domain := ""
	key := strings.ToUpper(county)
	if area != "" {
		key += "_" + strings.ToUpper(area)
	}
	// See https://transparency.entsoe.eu/content/static_content/Static%20content/web%20api/Guide.html#_areas
	switch key {
	case "DE_50HZ":
		domain = "10YDE-VE-------2"
	case "AL":
		domain = "10YAL-KESH-----5"
	case "DE_AMPRION":
		domain = "10YDE-RWENET---I"
	case "AT":
		domain = "10YAT-APG------L"
	case "BY":
		domain = "10Y1001A1001A51S"
	case "BE":
		domain = "10YBE----------2"
	case "BA":
		domain = "10YBA-JPCC-----D"
	case "BG":
		domain = "10YCA-BULGARIA-R"
	case "CZ_DE_SK":
		domain = "10YDOM-CZ-DE-SKK"
	case "HR":
		domain = "10YHR-HEP------M"
	case "CWE":
		domain = "10YDOM-REGION-1V"
	case "CY":
		domain = "10YCY-1001A0003J"
	case "CZ":
		domain = "10YCZ-CEPS-----N"
	case "DE_AT_LU":
		domain = "10Y1001A1001A63L"
	case "DE_LU":
		domain = "10Y1001A1001A82H"
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
	case "FI":
		domain = "10YFI-1--------U"
	case "MK":
		domain = "10YMK-MEPSO----8"
	case "FR":
		domain = "10YFR-RTE------C"
	case "DE":
		domain = "10Y1001A1001A83F"
	case "GR":
		domain = "10YGR-HTSO-----Y"
	case "HU":
		domain = "10YHU-MAVIR----U"
	case "IS":
		domain = "IS"
	case "IE_SEM":
		domain = "10Y1001A1001A59C"
	case "IE":
		domain = "10YIE-1001A00010"
	case "IT":
		domain = "10YIT-GRTN-----B"
	case "IT_SACO_AC":
		domain = "10Y1001A1001A885"
	case "IT_CALA":
		domain = "10Y1001C--00096J"
	case "IT_SACO_DC":
		domain = "10Y1001A1001A893"
	case "IT_BRNN":
		domain = "10Y1001A1001A699"
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
	case "IT_SARD":
		domain = "10Y1001A1001A74G"
	case "IT_SICI":
		domain = "10Y1001A1001A75E"
	case "IT_SUD":
		domain = "10Y1001A1001A788"
	case "RU_KGD":
		domain = "10Y1001A1001A50U"
	case "LV":
		domain = "10YLV-1001A00074"
	case "LT":
		domain = "10YLT-1001A0008Q"
	case "LU":
		domain = "10YLU-CEGEDEL-NQ"
	case "MT":
		domain = "10Y1001A1001A93C"
	case "ME":
		domain = "10YCS-CG-TSO---S"
	case "GB":
		domain = "10YGB----------A"
	case "GB_IFA":
		domain = "10Y1001C--00098F"
	case "GB_IFA2":
		domain = "17Y0000009369493"
	case "GB_ELECLINK":
		domain = "11Y0-0000-0265-K"
	case "UK":
		domain = "10Y1001A1001A92E"
	case "NL":
		domain = "10YNL----------L"
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
	case "NO":
		domain = "10YNO-0--------C"
	case "PL_CZ":
		domain = "10YDOM-1001A082L"
	case "PL":
		domain = "10YPL-AREA-----S"
	case "PT":
		domain = "10YPT-REN------W"
	case "MD":
		domain = "10Y1001A1001A990"
	case "RO":
		domain = "10YRO-TEL------P"
	case "RU":
		domain = "10Y1001A1001A49F"
	case "SE_1":
		domain = "10Y1001A1001A44P"
	case "SE_2":
		domain = "10Y1001A1001A45N"
	case "SE_3":
		domain = "10Y1001A1001A46L"
	case "SE_4":
		domain = "10Y1001A1001A47J"
	case "RS":
		domain = "10YCS-SERBIATSOV"
	case "SK":
		domain = "10YSK-SEPS-----K"
	case "SI":
		domain = "10YSI-ELES-----O"
	case "GB_NIR":
		domain = "10Y1001A1001A016"
	case "ES":
		domain = "10YES-REE------0"
	case "SE":
		domain = "10YSE-1--------K"
	case "CH":
		domain = "10YCH-SWISSGRIDZ"
	case "DE_TENNET":
		domain = "10YDE-EON------1"
	case "DE_TRANSNET":
		domain = "10YDE-ENBW-----N"
	case "TR":
		domain = "10YTR-TEIAS----W"
	case "UA":
		domain = "10Y1001C--00003F"
	case "UA_DOBTPP":
		domain = "10Y1001A1001A869"
	case "UA_BEI":
		domain = "10YUA-WEPS-----0"
	case "UA_IPS":
		domain = "10Y1001C--000182"
	case "XK":
		domain = "10Y1001C--00100H"
	default:
		return nil, fmt.Errorf("invalid country (%s) & area (%s) combination", county, area)
	}
	return &EntsoePriceImporter{
		domain:        domain,
		securityToken: securityToken,
	}, nil
}
